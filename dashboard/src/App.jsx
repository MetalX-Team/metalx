import { useEffect, useMemo, useState } from 'react'
import { Panel } from './components/Panel'
import { StatCard } from './components/StatCard'

const sections = ['总览', '节点', '节点详情', '终端与命令', '任务', '告警', '审计', '系统设置']
const apiBase = import.meta.env.VITE_API_BASE ?? ''
const wsBase = (() => {
  if (!apiBase) {
    return window.location.origin.replace(/^http/, 'ws')
  }
  if (apiBase.startsWith('https://')) {
    return apiBase.replace('https://', 'wss://')
  }
  if (apiBase.startsWith('http://')) {
    return apiBase.replace('http://', 'ws://')
  }
  return window.location.origin.replace(/^http/, 'ws')
})()
const credentials = {
  username: import.meta.env.VITE_METALX_USER ?? 'admin',
  password: import.meta.env.VITE_METALX_PASSWORD ?? 'metalx-admin-2026',
}

function pct(value) {
  return `${Number(value ?? 0).toFixed(1)}%`
}

function formatTime(value) {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}

function statusTone(value) {
  if (value === 'critical' || value === 'failed') return 'critical'
  if (value === 'warning' || value === 'partial') return 'warning'
  return 'success'
}

function statusText(value) {
  switch (value) {
    case 'critical':
      return '严重'
    case 'warning':
      return '警告'
    case 'normal':
      return '正常'
    case 'success':
      return '成功'
    case 'failed':
      return '失败'
    case 'completed':
      return '完成'
    case 'partial':
      return '部分成功'
    case 'running':
      return '执行中'
    case 'pending':
      return '等待中'
    default:
      return value || '-'
  }
}

function yesNo(value) {
  return value ? '启用' : '关闭'
}

async function request(path, token, options = {}) {
  const response = await fetch(`${apiBase}${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      Authorization: token,
      ...(options.headers ?? {}),
    },
  })

  if (!response.ok) {
    const message = await response.text()
    throw new Error(message || `${response.status} ${response.statusText}`)
  }

  return response.json()
}

export default function App() {
  const [activeSection, setActiveSection] = useState('总览')
  const [token, setToken] = useState('')
  const [summary, setSummary] = useState(null)
  const [nodes, setNodes] = useState([])
  const [selectedNodeId, setSelectedNodeId] = useState('')
  const [nodeDetail, setNodeDetail] = useState(null)
  const [tasks, setTasks] = useState([])
  const [audits, setAudits] = useState([])
  const [alerts, setAlerts] = useState([])
  const [system, setSystem] = useState(null)
  const [command, setCommand] = useState('uptime')
  const [selectedTargets, setSelectedTargets] = useState([])
  const [taskOutput, setTaskOutput] = useState(null)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [terminalInput, setTerminalInput] = useState('')
  const [terminalOutput, setTerminalOutput] = useState('')
  const [terminalConnected, setTerminalConnected] = useState(false)
  const [terminalStatus, setTerminalStatus] = useState('未连接')
  const [terminalSessionId, setTerminalSessionId] = useState('')
  const [terminalSocket, setTerminalSocket] = useState(null)

  useEffect(() => {
    let cancelled = false

    async function login() {
      try {
        const response = await fetch(`${apiBase}/api/auth/login`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(credentials),
        })
        if (!response.ok) {
          throw new Error(await response.text())
        }
        const payload = await response.json()
        if (!cancelled) {
          setToken(payload.token)
        }
      } catch (err) {
        if (!cancelled) {
          setLoading(false)
          setError(`登录失败：${err.message}`)
        }
      }
    }

    login()
    return () => {
      cancelled = true
    }
  }, [])

  useEffect(() => {
    if (!token) return
    let cancelled = false

    async function load() {
      try {
        const [summaryData, nodesData, tasksData, auditsData, alertsData, systemData] = await Promise.all([
          request('/api/summary', token),
          request('/api/nodes', token),
          request('/api/tasks', token),
          request('/api/audits', token),
          request('/api/alerts', token),
          request('/api/system', token),
        ])

        if (cancelled) return

        const nextNodes = nodesData.items ?? []
        setSummary(summaryData)
        setNodes(nextNodes)
        setTasks(tasksData.items ?? [])
        setAudits(auditsData.items ?? [])
        setAlerts(alertsData.items ?? [])
        setSystem(systemData)
        const preferredNode = nextNodes.find((node) => node.online) ?? nextNodes[0]
        setSelectedNodeId((current) => current || preferredNode?.id || '')
        setSelectedTargets((current) => current.length > 0 ? current : (preferredNode?.id ? [preferredNode.id] : []))
        setLoading(false)
        setError('')
      } catch (err) {
        if (!cancelled) {
          setLoading(false)
          setError(`加载失败：${err.message}`)
        }
      }
    }

    load()
    const timer = window.setInterval(load, 10000)
    return () => {
      cancelled = true
      window.clearInterval(timer)
    }
  }, [token])

  useEffect(() => {
    if (!token || !selectedNodeId) return
    let cancelled = false

    async function loadDetail() {
      try {
        const payload = await request(`/api/nodes/${selectedNodeId}`, token)
        if (!cancelled) {
          setNodeDetail(payload)
        }
      } catch (err) {
        if (!cancelled) {
          setError(`节点详情加载失败：${err.message}`)
        }
      }
    }

    loadDetail()
    const timer = window.setInterval(loadDetail, 10000)
    return () => {
      cancelled = true
      window.clearInterval(timer)
    }
  }, [token, selectedNodeId])

  useEffect(() => () => {
    if (terminalSocket) {
      terminalSocket.close()
    }
  }, [terminalSocket])

  const selectedNode = useMemo(
    () => nodes.find((node) => node.id === selectedNodeId) ?? null,
    [nodes, selectedNodeId],
  )
  const nodeSummary = nodeDetail?.summary ?? null

  async function runTask() {
    if (!token || !command.trim() || selectedTargets.length === 0) {
      return
    }
    setIsSubmitting(true)
    setError('')
    try {
      const payload = await request('/api/tasks', token, {
        method: 'POST',
        body: JSON.stringify({
          command: command.trim(),
          targets: selectedTargets,
          actor: 'dashboard',
        }),
      })
      setTaskOutput(payload)
      setActiveSection('任务')

      const [tasksData, auditsData] = await Promise.all([
        request('/api/tasks', token),
        request('/api/audits', token),
      ])
      setTasks(tasksData.items ?? [])
      setAudits(auditsData.items ?? [])
    } catch (err) {
      setError(`命令执行失败：${err.message}`)
    } finally {
      setIsSubmitting(false)
    }
  }

  function toggleTarget(nodeID) {
    setSelectedTargets((current) =>
      current.includes(nodeID) ? current.filter((item) => item !== nodeID) : [...current, nodeID],
    )
  }

  function openTerminal() {
    if (!token || !selectedNodeId) {
      return
    }
    if (terminalSocket) {
      terminalSocket.close()
    }
    const sessionId = `term-${Date.now()}`
    const socket = new WebSocket(`${wsBase}/api/terminal?token=${encodeURIComponent(token)}`)
    setTerminalStatus('连接中')
    socket.onopen = () => {
      setTerminalConnected(true)
      setTerminalStatus('已连接')
      setTerminalSessionId(sessionId)
      socket.send(JSON.stringify({
        nodeId: selectedNodeId,
        sessionId,
        shell: '/bin/bash',
        open: true,
        cols: 120,
        rows: 32,
      }))
    }
    socket.onmessage = (event) => {
      const frame = JSON.parse(event.data)
      if (frame.output) {
        setTerminalOutput((current) => `${current}${frame.output}`)
      }
      if (frame.close) {
        setTerminalConnected(false)
        setTerminalStatus('已关闭')
      }
      if (frame.error) {
        setError(frame.error)
      }
    }
    socket.onerror = () => {
      setTerminalStatus('错误')
      setTerminalConnected(false)
    }
    socket.onclose = () => {
      setTerminalConnected(false)
      setTerminalStatus('已关闭')
    }
    setTerminalSocket(socket)
    setTerminalOutput('')
    setActiveSection('终端与命令')
  }

  function closeTerminal() {
    if (terminalSocket && terminalConnected) {
      terminalSocket.send(JSON.stringify({ nodeId: selectedNodeId, sessionId: terminalSessionId, close: true }))
      terminalSocket.close()
    }
    setTerminalConnected(false)
    setTerminalStatus('已关闭')
  }

  function sendTerminalLine() {
    if (!terminalSocket || !terminalConnected || !terminalInput) {
      return
    }
    terminalSocket.send(JSON.stringify({
      nodeId: selectedNodeId,
      sessionId: terminalSessionId,
      input: `${terminalInput}\n`,
    }))
    setTerminalInput('')
  }

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="brand">
          <p className="eyebrow">MetalX</p>
          <h1>运维总控台</h1>
        </div>

        <nav className="nav-list">
          {sections.map((section) => (
            <button
              key={section}
              className={`nav-list__item${activeSection === section ? ' is-active' : ''}`}
              onClick={() => setActiveSection(section)}
            >
              {section}
            </button>
          ))}
        </nav>

        <div className="sidebar__foot">
          <span>接口地址：{apiBase}</span>
          <span>节点数量：{nodes.length}</span>
          <span>当前节点：{selectedNode?.name ?? '-'}</span>
        </div>
      </aside>

      <main className="content">
        <section className="hero">
          <div><h2>集群实时总览</h2></div>
          <div className="hero__actions">
            <button className="button button--primary" onClick={() => setActiveSection('终端与命令')}>
              执行命令
            </button>
            <button className="button" onClick={openTerminal}>
              打开终端
            </button>
          </div>
        </section>

        {error ? <div className="banner banner--error">{error}</div> : null}
        {loading ? <div className="banner">正在加载实时数据...</div> : null}

        {summary ? (
          <section className="stats-grid">
            <StatCard label="在线节点" value={`${summary.onlineNodes}/${summary.totalNodes}`} meta="在线 / 总数" />
            <StatCard label="CPU 平均" value={pct(summary.averageCPU)} meta="集群平均" />
            <StatCard label="内存平均" value={pct(summary.averageMemory)} meta="集群平均" />
            <StatCard label="磁盘平均" value={pct(summary.averageDisk)} meta="集群平均" />
            <StatCard label="当前告警" value={summary.alertCount} tone="warning" meta="告警数量" />
            <StatCard label="任务成功率" value={pct(summary.taskSuccessRate)} meta="历史任务" />
            <StatCard label="网络总量" value={`${Number(summary.networkThroughput ?? 0).toFixed(1)} MB`} meta="收发总量" />
            <StatCard label="最后更新" value={formatTime(summary.updatedAt)} meta="控制面时间" />
          </section>
        ) : null}

        <section className="content-grid">
          {(activeSection === '总览' || activeSection === '节点') && (
            <Panel title="节点总览">
              <div className="table-wrap">
                <table>
                  <thead>
                    <tr>
                      <th>节点</th>
                      <th>状态</th>
                      <th>系统</th>
                      <th>CPU</th>
                      <th>内存</th>
                      <th>磁盘</th>
                      <th>负载</th>
                      <th>进程</th>
                      <th>IP</th>
                      <th>告警</th>
                    </tr>
                  </thead>
                  <tbody>
                    {nodes.map((node) => (
                      <tr
                        key={node.id}
                        className={selectedNodeId === node.id ? 'row-active' : ''}
                        onClick={() => {
                          setSelectedNodeId(node.id)
                          setActiveSection('节点详情')
                        }}
                      >
                        <td>{node.name}</td>
                        <td><span className={`badge badge--${node.online ? 'success' : 'muted'}`}>{node.online ? '在线' : '离线'}</span></td>
                        <td>{node.os}</td>
                        <td>{pct(node.cpuUsage)}</td>
                        <td>{pct(node.memoryUsage)}</td>
                        <td>{pct(node.diskUsage)}</td>
                        <td>{Number(node.load1 ?? 0).toFixed(2)}</td>
                        <td>{node.processCount}</td>
                        <td>{node.ipAddress}</td>
                        <td><span className={`badge badge--${statusTone(node.alertLevel)}`}>{statusText(node.alertLevel)}</span></td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </Panel>
          )}

          {(activeSection === '总览' || activeSection === '节点详情') && nodeDetail && (
            <Panel title="节点详情" subtitle={selectedNode?.name ?? selectedNodeId}>
              <div className="overview-grid">
                <div className="mini-stat"><span>系统</span><strong>{nodeSummary?.os || '-'}</strong></div>
                <div className="mini-stat"><span>内核</span><strong>{nodeSummary?.kernel || '-'}</strong></div>
                <div className="mini-stat"><span>在线时间</span><strong>{nodeDetail.uptime || '-'}</strong></div>
                <div className="mini-stat"><span>主 IP</span><strong>{nodeSummary?.ipAddress || '-'}</strong></div>
                <div className="mini-stat"><span>MAC</span><strong>{nodeSummary?.macAddress || '-'}</strong></div>
                <div className="mini-stat"><span>用户数</span><strong>{nodeDetail.userCount ?? 0}</strong></div>
              </div>

              <div className="chart-strip">
                {[
                  { label: 'CPU', value: nodeSummary?.cpuUsage },
                  { label: '内存', value: nodeSummary?.memoryUsage },
                  { label: '磁盘', value: nodeSummary?.diskUsage },
                  { label: '负载', value: Math.min((nodeSummary?.load1 ?? 0) * 20, 100) },
                ].map((item) => (
                  <div key={item.label} className="meter">
                    <div className="meter__label">
                      <span>{item.label}</span>
                      <strong>{item.label === '负载' ? Number(nodeSummary?.load1 ?? 0).toFixed(2) : pct(item.value)}</strong>
                    </div>
                    <div className="meter__track">
                      <div className="meter__fill" style={{ width: `${Math.min(Number(item.value ?? 0), 100)}%` }} />
                    </div>
                  </div>
                ))}
              </div>

              <div className="dual-list">
                <div>
                  <h3>网络接口</h3>
                  {nodeDetail.interfaces?.map((item) => (
                    <div className="list-row" key={item.name}>
                      <strong>{item.name}</strong>
                      <span>{item.ip || '-'}</span>
                      <span>{item.state}</span>
                      <span>{item.rx} / {item.tx}</span>
                    </div>
                  ))}
                </div>
                <div>
                  <h3>文件系统</h3>
                  {nodeDetail.filesystems?.map((item) => (
                    <div className="list-row" key={item.mount}>
                      <strong>{item.mount}</strong>
                      <span>{item.size}</span>
                      <span>{pct(item.usedPercent)}</span>
                      <span />
                    </div>
                  ))}
                </div>
              </div>
            </Panel>
          )}

          {(activeSection === '终端与命令' || activeSection === '总览') && (
            <Panel title="批量命令">
              <div className="control-stack">
                <label className="field">
                  <span>命令</span>
                  <textarea value={command} onChange={(event) => setCommand(event.target.value)} rows={4} />
                </label>
                <div className="target-list">
                  {nodes.map((node) => (
                    <label key={node.id} className="target-chip">
                      <input
                        type="checkbox"
                        checked={selectedTargets.includes(node.id)}
                        onChange={() => toggleTarget(node.id)}
                      />
                      <span>{node.name}</span>
                    </label>
                  ))}
                </div>
                <div className="hero__actions">
                  <button className="button button--primary" onClick={runTask} disabled={isSubmitting}>
                    {isSubmitting ? '执行中...' : '执行命令'}
                  </button>
                </div>
                {taskOutput ? (
                  <div className="terminal-output">
                    <div className="terminal-output__title">最近结果：{taskOutput.id}</div>
                    {taskOutput.results?.map((result) => (
                      <div key={`${taskOutput.id}-${result.nodeId}`} className="result-block">
                        <div className="result-block__meta">
                          <strong>{result.nodeId}</strong>
                          <span>{statusText(result.status)}</span>
                          <span>退出码 {result.exitCode}</span>
                          <span>{result.duration}</span>
                        </div>
                        <pre>{result.stdout || '无标准输出'}</pre>
                        {result.stderr ? <pre className="stderr">{result.stderr}</pre> : null}
                      </div>
                    ))}
                  </div>
                ) : null}
              </div>
            </Panel>
          )}

          {activeSection === '终端与命令' && (
            <Panel title="流式终端" subtitle={`当前节点：${selectedNode?.name ?? selectedNodeId}`}>
              <div className="control-stack">
                <div className="hero__actions">
                  <button className="button button--primary" onClick={openTerminal} disabled={terminalConnected}>
                    {terminalConnected ? '终端已连接' : '连接终端'}
                  </button>
                  <button className="button" onClick={closeTerminal} disabled={!terminalConnected}>
                    关闭终端
                  </button>
                  <span className="terminal-status">状态：{terminalStatus}</span>
                </div>
                <pre className="terminal-screen">{terminalOutput || '$ '}</pre>
                <label className="field">
                  <span>输入命令</span>
                  <input
                    className="terminal-input"
                    value={terminalInput}
                    onChange={(event) => setTerminalInput(event.target.value)}
                    onKeyDown={(event) => {
                      if (event.key === 'Enter') {
                        event.preventDefault()
                        sendTerminalLine()
                      }
                    }}
                    placeholder="例如：pwd"
                  />
                </label>
                <div className="hero__actions">
                  <button className="button button--primary" onClick={sendTerminalLine} disabled={!terminalConnected}>
                    发送
                  </button>
                  <button
                    className="button"
                    onClick={() => terminalSocket?.send(JSON.stringify({ nodeId: selectedNodeId, sessionId: terminalSessionId, input: '\u0003' }))}
                    disabled={!terminalConnected}
                  >
                    中断
                  </button>
                </div>
              </div>
            </Panel>
          )}

          {activeSection === '任务' && (
            <Panel title="任务历史">
              {tasks.map((task) => (
                <div className="task-card" key={task.id}>
                  <div className="task-card__head">
                    <strong>{task.command}</strong>
                    <span className={`badge badge--${statusTone(task.status)}`}>{statusText(task.status)}</span>
                    <span>{formatTime(task.startedAt)}</span>
                  </div>
                  {task.results?.map((result) => (
                    <div className="list-row" key={`${task.id}-${result.nodeId}`}>
                      <strong>{result.nodeId}</strong>
                      <span>{statusText(result.status)}</span>
                      <span>退出码 {result.exitCode}</span>
                      <span>{result.duration}</span>
                    </div>
                  ))}
                </div>
              ))}
            </Panel>
          )}

          {activeSection === '告警' && (
            <Panel title="当前告警">
              {alerts.length === 0 ? <div className="empty-state">当前没有告警</div> : null}
              {alerts.map((alert, index) => (
                <div className="list-row" key={`${alert.nodeId}-${index}`}>
                  <strong>{alert.nodeName}</strong>
                  <span>{alert.message}</span>
                  <span className={`badge badge--${statusTone(alert.severity)}`}>{statusText(alert.severity)}</span>
                  <span>{formatTime(alert.at)}</span>
                </div>
              ))}
            </Panel>
          )}

          {activeSection === '审计' && (
            <Panel title="审计记录">
              {audits.map((audit) => (
                <div className="list-row" key={audit.id}>
                  <strong>{audit.actor}</strong>
                  <span>{audit.action}</span>
                  <span>{audit.target}</span>
                  <span>{formatTime(audit.createdAt)}</span>
                </div>
              ))}
            </Panel>
          )}

          {activeSection === '系统设置' && system && (
            <Panel title="系统设置">
              <div className="overview-grid">
                <div className="mini-stat"><span>控制地址</span><strong>{system.controllerAddress}</strong></div>
                <div className="mini-stat"><span>发现端口</span><strong>{system.discoveryPort}</strong></div>
                <div className="mini-stat"><span>终端开关</span><strong>{yesNo(system.shellEnabled)}</strong></div>
                <div className="mini-stat"><span>存储类型</span><strong>{system.store}</strong></div>
                <div className="mini-stat"><span>数据库路径</span><strong>{system.databasePath}</strong></div>
                <div className="mini-stat"><span>系统时间</span><strong>{formatTime(system.timestamp)}</strong></div>
              </div>
            </Panel>
          )}

          {(activeSection === '节点详情' || activeSection === '总览') && nodeDetail && (
            <Panel title="进程 / 登录用户 / 最近命令">
              <div className="triple-stack">
                <div>
                  <h3>高占用进程</h3>
                  {nodeDetail.topProcesses?.map((process) => (
                    <div className="list-row" key={process.pid}>
                      <strong>{process.name}</strong>
                      <span>PID {process.pid}</span>
                      <span>CPU {pct(process.cpu)}</span>
                      <span>内存 {pct(process.mem)}</span>
                    </div>
                  ))}
                </div>
                <div>
                  <h3>登录用户</h3>
                  {nodeDetail.loggedUsers?.length === 0 ? <div className="empty-state">当前没有登录会话</div> : null}
                  {nodeDetail.loggedUsers?.map((user, index) => (
                    <div className="list-row" key={`${user.user}-${index}`}>
                      <strong>{user.user}</strong>
                      <span>{user.tty}</span>
                      <span>{user.from}</span>
                      <span />
                    </div>
                  ))}
                </div>
                <div>
                  <h3>最近命令</h3>
                  {nodeDetail.recentCommands?.length === 0 ? <div className="empty-state">暂无命令记录</div> : null}
                  {nodeDetail.recentCommands?.map((result, index) => (
                    <div className="list-row" key={`${result.nodeId}-${index}`}>
                      <strong>{result.nodeId}</strong>
                      <span>{statusText(result.status)}</span>
                      <span>退出码 {result.exitCode}</span>
                      <span>{result.startedAt}</span>
                    </div>
                  ))}
                </div>
              </div>
            </Panel>
          )}
        </section>
      </main>
    </div>
  )
}
