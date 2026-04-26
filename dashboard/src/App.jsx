import { useEffect, useMemo, useState } from 'react'
import { Panel } from './components/Panel'
import { StatCard } from './components/StatCard'

const sections = ['总览', '节点', '节点详情', '终端与命令', '任务', '告警', '审计', '系统设置', '装机模板', '装机任务']
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

function toDnsmasqDraft(settings) {
  return {
    enabled: settings?.enabled ?? false,
    listenInterface: settings?.listenInterface ?? 'eth0',
    bindAddress: settings?.bindAddress ?? '',
    dhcpRangeStart: settings?.dhcpRangeStart ?? '',
    dhcpRangeEnd: settings?.dhcpRangeEnd ?? '',
    dhcpLeaseTime: settings?.dhcpLeaseTime ?? '12h',
    gateway: settings?.gateway ?? '',
    dnsServersText: (settings?.dnsServers ?? []).join(', '),
    tftpRoot: settings?.tftpRoot ?? '',
    bootFile: settings?.bootFile ?? 'pxelinux.0',
    pxePrompt: settings?.pxePrompt ?? '',
    pxeServiceLabel: settings?.pxeServiceLabel ?? '',
    kernelPath: settings?.kernelPath ?? '',
    initrdPath: settings?.initrdPath ?? '',
    bootArgs: settings?.bootArgs ?? '',
    nextServer: settings?.nextServer ?? '',
  }
}

function parseDnsServers(value) {
  return value
    .split(/[\n,]/)
    .map((item) => item.trim())
    .filter(Boolean)
}

function parseLines(value) {
  return value
    .split('\n')
    .map((item) => item.trim())
    .filter(Boolean)
}

function toInstallProfileDraft(profile) {
  return {
    id: profile?.id ?? '',
    name: profile?.name ?? '',
    osFamily: profile?.osFamily ?? 'ubuntu',
    osVersion: profile?.osVersion ?? '',
    architecture: profile?.architecture ?? 'amd64',
    firmware: profile?.firmware ?? 'uefi',
    installSource: profile?.installSource ?? '',
    bootKernelPath: profile?.bootKernelPath ?? '',
    bootInitrdPath: profile?.bootInitrdPath ?? '',
    hostnamePattern: profile?.hostnamePattern ?? '',
    timezone: profile?.timezone ?? 'Etc/UTC',
    locale: profile?.locale ?? 'en_US.UTF-8',
    keyboardLayout: profile?.keyboardLayout ?? 'us',
    adminUsername: profile?.adminUsername ?? 'metalx',
    adminPasswordHash: profile?.adminPasswordHash ?? '',
    sshAuthorizedKeysText: (profile?.sshAuthorizedKeys ?? []).join('\n'),
    packagesText: (profile?.packages ?? []).join('\n'),
    packageMirror: profile?.packageMirror ?? '',
    diskLayout: profile?.diskLayout ?? '',
    networkMode: profile?.networkMode ?? 'dhcp',
    agentBinaryUrl: profile?.agentBinaryUrl ?? '',
    agentServiceName: profile?.agentServiceName ?? 'metalx-agent',
    controllerGrpcAddress: profile?.controllerGrpcAddress ?? '',
    extraKernelArgs: profile?.extraKernelArgs ?? '',
    postInstallScript: profile?.postInstallScript ?? '',
    enabled: profile?.enabled ?? true,
  }
}

function toRuntimeDraft(settings) {
  return {
    allowShell: settings?.allowShell ?? true,
    discoveryPort: settings?.discoveryPort ?? 9527,
    dnsmasqStateDir: settings?.dnsmasqStateDir ?? '',
    provisioningBaseUrl: settings?.provisioningBaseUrl ?? '',
    publicGrpcAddress: settings?.publicGrpcAddress ?? '',
    agentBinaryPath: settings?.agentBinaryPath ?? '',
    defaultNodeAddr: settings?.defaultNodeAddr ?? '',
    dashboardRefreshIntervalMs: settings?.dashboardRefreshIntervalMs ?? 1000,
    dashboardDefaultCommand: settings?.dashboardDefaultCommand ?? 'uptime',
    terminalShell: settings?.terminalShell ?? '/bin/bash',
    agentListenAddress: settings?.agentListenAddress ?? ':18081',
    agentGrpcListenAddress: settings?.agentGrpcListenAddress ?? ':19091',
    agentReportIntervalSeconds: settings?.agentReportIntervalSeconds ?? 1,
    adminUser: settings?.adminUser ?? '',
    adminPassword: '',
  }
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
  const [loginUsername, setLoginUsername] = useState('')
  const [loginPassword, setLoginPassword] = useState('')
  const [isLoggingIn, setIsLoggingIn] = useState(false)
  const [summary, setSummary] = useState(null)
  const [nodes, setNodes] = useState([])
  const [selectedNodeId, setSelectedNodeId] = useState('')
  const [nodeDetail, setNodeDetail] = useState(null)
  const [tasks, setTasks] = useState([])
  const [audits, setAudits] = useState([])
  const [alerts, setAlerts] = useState([])
  const [system, setSystem] = useState(null)
  const [command, setCommand] = useState('')
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
  const [lastRefreshAt, setLastRefreshAt] = useState('')
  const [isRefreshing, setIsRefreshing] = useState(false)
  const [dnsmasqSettings, setDnsmasqSettings] = useState(null)
  const [runtimeSettings, setRuntimeSettings] = useState(null)
  const [runtimeDraft, setRuntimeDraft] = useState(() => toRuntimeDraft(null))
  const [runtimeDirty, setRuntimeDirty] = useState(false)
  const [runtimeSaving, setRuntimeSaving] = useState(false)
  const [dnsmasqDraft, setDnsmasqDraft] = useState(() => toDnsmasqDraft(null))
  const [dnsmasqDirty, setDnsmasqDirty] = useState(false)
  const [dnsmasqSaving, setDnsmasqSaving] = useState(false)
  const [installProfiles, setInstallProfiles] = useState([])
  const [installJobs, setInstallJobs] = useState([])
  const [selectedProfileId, setSelectedProfileId] = useState('')
  const [profileDraft, setProfileDraft] = useState(() => toInstallProfileDraft(null))
  const [profileDirty, setProfileDirty] = useState(false)
  const [profileSaving, setProfileSaving] = useState(false)
  const [jobDraft, setJobDraft] = useState({ profileId: '', macAddress: '', hostname: '', nodeId: '' })
  const [jobSubmitting, setJobSubmitting] = useState(false)
  const refreshIntervalMs = Math.max(Number(runtimeSettings?.dashboardRefreshIntervalMs ?? 1000), 500)
  const sectionTitle = activeSection === '总览' ? '集群实时总览' : activeSection

  useEffect(() => {
    setLoading(false)
  }, [])

  useEffect(() => {
    if (!token) return
    let cancelled = false
    let timerId = 0
    let inFlight = false

    async function load() {
      if (cancelled || inFlight) {
        return
      }

      inFlight = true
      setIsRefreshing(true)
      try {
        const [summaryData, nodesData, tasksData, auditsData, alertsData, systemData, runtimeData, dnsmasqData, profilesData, jobsData] = await Promise.all([
          request('/api/summary', token),
          request('/api/nodes', token),
          request('/api/tasks', token),
          request('/api/audits', token),
          request('/api/alerts', token),
          request('/api/system', token),
          request('/api/settings/runtime', token),
          request('/api/system/dnsmasq', token),
          request('/api/install/profiles', token),
          request('/api/install/jobs', token),
        ])

        if (cancelled) return

        const nextNodes = nodesData.items ?? []
        const resolvedNodeId = nextNodes.some((node) => node.id === selectedNodeId)
          ? selectedNodeId
          : nextNodes.find((node) => node.online)?.id
            || nextNodes[0]?.id
            || ''
        const detailData = resolvedNodeId
          ? await request(`/api/nodes/${resolvedNodeId}`, token)
          : null

        if (cancelled) return

        setSummary(summaryData)
        setNodes(nextNodes)
        setTasks(tasksData.items ?? [])
        setAudits(auditsData.items ?? [])
        setAlerts(alertsData.items ?? [])
        setSystem(systemData)
        setRuntimeSettings(runtimeData)
        setDnsmasqSettings(dnsmasqData)
        setInstallProfiles(profilesData.items ?? [])
        setInstallJobs(jobsData.items ?? [])
        setNodeDetail(detailData)
        setSelectedNodeId(resolvedNodeId)
        setSelectedTargets((current) => {
          const nextSelected = current.filter((target) => nextNodes.some((node) => node.id === target))
          if (nextSelected.length > 0) {
            return nextSelected
          }
          return resolvedNodeId ? [resolvedNodeId] : []
        })
        setLastRefreshAt(new Date().toISOString())
        setLoading(false)
        setError('')
      } catch (err) {
        if (!cancelled) {
          setLoading(false)
          setError(`加载失败：${err.message}`)
        }
      } finally {
        inFlight = false
        if (!cancelled) {
          setIsRefreshing(false)
          timerId = window.setTimeout(load, refreshIntervalMs)
        }
      }
    }

    load()
    return () => {
      cancelled = true
      window.clearTimeout(timerId)
    }
  }, [token, selectedNodeId, refreshIntervalMs])

  useEffect(() => {
    if (!dnsmasqSettings || dnsmasqDirty) return
    setDnsmasqDraft(toDnsmasqDraft(dnsmasqSettings))
  }, [dnsmasqSettings, dnsmasqDirty])

  useEffect(() => {
    if (!runtimeSettings || runtimeDirty) return
    setRuntimeDraft(toRuntimeDraft(runtimeSettings))
    setCommand((current) => current || runtimeSettings.dashboardDefaultCommand || 'uptime')
  }, [runtimeSettings, runtimeDirty])

  useEffect(() => {
    if (installProfiles.length === 0) return
    const nextSelected = installProfiles.find((item) => item.id === selectedProfileId) ?? installProfiles[0]
    if (!selectedProfileId || nextSelected.id !== selectedProfileId) {
      setSelectedProfileId(nextSelected.id)
    }
    if (!profileDirty) {
      setProfileDraft(toInstallProfileDraft(nextSelected))
    }
    setJobDraft((current) => ({
      ...current,
      profileId: current.profileId || nextSelected.id,
    }))
  }, [installProfiles, selectedProfileId, profileDirty])

  useEffect(() => () => {
    if (terminalSocket) {
      terminalSocket.close()
    }
  }, [terminalSocket])

  const selectedNode = useMemo(
    () => nodes.find((node) => node.id === selectedNodeId) ?? null,
    [nodes, selectedNodeId],
  )
  const selectedProfile = useMemo(
    () => installProfiles.find((item) => item.id === selectedProfileId) ?? null,
    [installProfiles, selectedProfileId],
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

  async function login() {
    if (!loginUsername.trim() || !loginPassword) {
      return
    }
    setIsLoggingIn(true)
    setError('')
    try {
      const response = await fetch(`${apiBase}/api/auth/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          username: loginUsername.trim(),
          password: loginPassword,
        }),
      })
      if (!response.ok) {
        throw new Error(await response.text())
      }
      const payload = await response.json()
      setToken(payload.token)
      setLoginPassword('')
      setLoading(true)
    } catch (err) {
      setError(`登录失败：${err.message}`)
    } finally {
      setIsLoggingIn(false)
    }
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
        shell: runtimeSettings?.terminalShell || '/bin/bash',
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

  function updateDnsmasqDraft(field, value) {
    setDnsmasqDirty(true)
    setDnsmasqDraft((current) => ({ ...current, [field]: value }))
  }

  function updateRuntimeDraft(field, value) {
    setRuntimeDirty(true)
    setRuntimeDraft((current) => ({ ...current, [field]: value }))
  }

  async function saveRuntimeSettings() {
    if (!token) return
    setRuntimeSaving(true)
    setError('')
    const shouldRelogin = Boolean(runtimeDraft.adminPassword)
    try {
      const payload = await request('/api/settings/runtime', token, {
        method: 'PUT',
        body: JSON.stringify({
          ...runtimeDraft,
          discoveryPort: Number(runtimeDraft.discoveryPort),
          dashboardRefreshIntervalMs: Number(runtimeDraft.dashboardRefreshIntervalMs),
          agentReportIntervalSeconds: Number(runtimeDraft.agentReportIntervalSeconds),
          actor: 'dashboard',
        }),
      })
      setRuntimeSettings(payload)
      setRuntimeDraft(toRuntimeDraft(payload))
      setRuntimeDirty(false)
      if (payload.dashboardDefaultCommand) {
        setCommand(payload.dashboardDefaultCommand)
      }
      if (payload.adminUser) {
        setLoginUsername(payload.adminUser)
      }
      if (shouldRelogin) {
        setToken('')
        setLoginPassword('')
      }
    } catch (err) {
      setError(`运行配置保存失败：${err.message}`)
    } finally {
      setRuntimeSaving(false)
    }
  }

  async function saveDnsmasqSettings() {
    if (!token) return
    setDnsmasqSaving(true)
    setError('')
    try {
      const payload = await request('/api/system/dnsmasq', token, {
        method: 'PUT',
        body: JSON.stringify({
          enabled: dnsmasqDraft.enabled,
          listenInterface: dnsmasqDraft.listenInterface.trim(),
          bindAddress: dnsmasqDraft.bindAddress.trim(),
          dhcpRangeStart: dnsmasqDraft.dhcpRangeStart.trim(),
          dhcpRangeEnd: dnsmasqDraft.dhcpRangeEnd.trim(),
          dhcpLeaseTime: dnsmasqDraft.dhcpLeaseTime.trim(),
          gateway: dnsmasqDraft.gateway.trim(),
          dnsServers: parseDnsServers(dnsmasqDraft.dnsServersText),
          tftpRoot: dnsmasqDraft.tftpRoot.trim(),
          bootFile: dnsmasqDraft.bootFile.trim(),
          pxePrompt: dnsmasqDraft.pxePrompt.trim(),
          pxeServiceLabel: dnsmasqDraft.pxeServiceLabel.trim(),
          kernelPath: dnsmasqDraft.kernelPath.trim(),
          initrdPath: dnsmasqDraft.initrdPath.trim(),
          bootArgs: dnsmasqDraft.bootArgs.trim(),
          nextServer: dnsmasqDraft.nextServer.trim(),
          actor: 'dashboard',
        }),
      })
      setDnsmasqSettings(payload)
      setDnsmasqDraft(toDnsmasqDraft(payload))
      setDnsmasqDirty(false)
    } catch (err) {
      setError(`dnsmasq 配置保存失败：${err.message}`)
    } finally {
      setDnsmasqSaving(false)
    }
  }

  function updateProfileDraft(field, value) {
    setProfileDirty(true)
    setProfileDraft((current) => ({ ...current, [field]: value }))
  }

  async function saveInstallProfile() {
    if (!token) return
    setProfileSaving(true)
    setError('')
    try {
      const payload = await request('/api/install/profiles', token, {
        method: 'PUT',
        body: JSON.stringify({
          ...profileDraft,
          sshAuthorizedKeys: parseLines(profileDraft.sshAuthorizedKeysText),
          packages: parseLines(profileDraft.packagesText),
          actor: 'dashboard',
        }),
      })
      setInstallProfiles((current) => {
        const rest = current.filter((item) => item.id !== payload.id)
        return [...rest, payload].sort((left, right) => left.name.localeCompare(right.name, 'zh-CN'))
      })
      setSelectedProfileId(payload.id)
      setProfileDraft(toInstallProfileDraft(payload))
      setProfileDirty(false)
      setJobDraft((current) => ({ ...current, profileId: payload.id }))
    } catch (err) {
      setError(`装机模板保存失败：${err.message}`)
    } finally {
      setProfileSaving(false)
    }
  }

  async function createInstallJob() {
    if (!token || !jobDraft.profileId || !jobDraft.macAddress.trim()) {
      return
    }
    setJobSubmitting(true)
    setError('')
    try {
      const payload = await request('/api/install/jobs', token, {
        method: 'POST',
        body: JSON.stringify({
          profileId: jobDraft.profileId,
          macAddress: jobDraft.macAddress.trim(),
          hostname: jobDraft.hostname.trim(),
          nodeId: jobDraft.nodeId.trim(),
          actor: 'dashboard',
        }),
      })
      setInstallJobs((current) => [payload, ...current.filter((item) => item.id !== payload.id)])
      setJobDraft((current) => ({ ...current, macAddress: '', hostname: '', nodeId: '' }))
      setActiveSection('装机任务')
    } catch (err) {
      setError(`装机任务创建失败：${err.message}`)
    } finally {
      setJobSubmitting(false)
    }
  }

  if (!token) {
    return (
      <div className="app-shell app-shell--auth">
        <main className="content content--auth">
          <section className="hero hero--auth">
            <div>
              <p className="eyebrow">Control Plane Access</p>
              <h2>MetalX 登录</h2>
              <p className="hero__meta">
                登录后可在系统设置中修改运行配置与管理员凭证
              </p>
            </div>
          </section>
          {error ? <div className="banner banner--error">{error}</div> : null}
          <section className="content-grid">
            <Panel title="管理员登录">
              <div className="control-stack">
                <label className="field">
                  <span>用户名</span>
                  <input className="text-input" value={loginUsername} onChange={(event) => setLoginUsername(event.target.value)} />
                </label>
                <label className="field">
                  <span>密码</span>
                  <input
                    className="text-input"
                    type="password"
                    value={loginPassword}
                    onChange={(event) => setLoginPassword(event.target.value)}
                    onKeyDown={(event) => {
                      if (event.key === 'Enter') {
                        event.preventDefault()
                        login()
                      }
                    }}
                  />
                </label>
                <div className="hero__actions">
                  <button className="button button--primary" onClick={login} disabled={isLoggingIn}>
                    {isLoggingIn ? '登录中...' : '登录'}
                  </button>
                </div>
              </div>
            </Panel>
          </section>
        </main>
      </div>
    )
  }

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="brand">
          <p className="eyebrow">MetalX</p>
          <h1>运维总控台</h1>
          <p className="sidebar__copy">白色工作台视图，聚焦节点、装机和实时执行路径。</p>
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
          <div className="sidebar__meta-card">
            <span>接口地址</span>
            <strong>{apiBase || window.location.origin}</strong>
          </div>
          <div className="sidebar__meta-card">
            <span>节点数量</span>
            <strong>{nodes.length}</strong>
          </div>
          <div className="sidebar__meta-card">
            <span>当前节点</span>
            <strong>{selectedNode?.name ?? '-'}</strong>
          </div>
        </div>
      </aside>

      <main className="content">
        <section className="hero">
          <div>
            <p className="eyebrow">Live Operations</p>
            <h2>{sectionTitle}</h2>
            <p className="hero__meta">
              <span>当前视图：{activeSection}</span>
              <span>刷新频率：{(refreshIntervalMs / 1000).toFixed(refreshIntervalMs % 1000 === 0 ? 0 : 1)} 秒</span>
              <span>状态：{isRefreshing ? '刷新中' : '实时更新中'}</span>
              <span>最近刷新：{formatTime(lastRefreshAt)}</span>
            </p>
          </div>
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

              {runtimeSettings ? (
                <div className="control-stack">
                  <div className="panel-subtitle">运行配置</div>
                  <div className="form-grid">
                    <label className="field">
                      <span>允许命令执行</span>
                      <label className="toggle">
                        <input type="checkbox" checked={runtimeDraft.allowShell} onChange={(event) => updateRuntimeDraft('allowShell', event.target.checked)} />
                        <span>{runtimeDraft.allowShell ? '启用' : '关闭'}</span>
                      </label>
                    </label>
                    <label className="field">
                      <span>发现端口</span>
                      <input className="text-input" value={runtimeDraft.discoveryPort} onChange={(event) => updateRuntimeDraft('discoveryPort', event.target.value)} />
                    </label>
                    <label className="field field--full">
                      <span>Provisioning Base URL</span>
                      <input className="text-input" value={runtimeDraft.provisioningBaseUrl} onChange={(event) => updateRuntimeDraft('provisioningBaseUrl', event.target.value)} />
                    </label>
                    <label className="field">
                      <span>Public gRPC 地址</span>
                      <input className="text-input" value={runtimeDraft.publicGrpcAddress} onChange={(event) => updateRuntimeDraft('publicGrpcAddress', event.target.value)} />
                    </label>
                    <label className="field">
                      <span>默认节点地址</span>
                      <input className="text-input" value={runtimeDraft.defaultNodeAddr} onChange={(event) => updateRuntimeDraft('defaultNodeAddr', event.target.value)} />
                    </label>
                    <label className="field field--full">
                      <span>Agent 二进制路径</span>
                      <input className="text-input" value={runtimeDraft.agentBinaryPath} onChange={(event) => updateRuntimeDraft('agentBinaryPath', event.target.value)} />
                    </label>
                    <label className="field field--full">
                      <span>dnsmasq 状态目录</span>
                      <input className="text-input" value={runtimeDraft.dnsmasqStateDir} onChange={(event) => updateRuntimeDraft('dnsmasqStateDir', event.target.value)} />
                    </label>
                    <label className="field">
                      <span>Dashboard 刷新间隔(ms)</span>
                      <input className="text-input" value={runtimeDraft.dashboardRefreshIntervalMs} onChange={(event) => updateRuntimeDraft('dashboardRefreshIntervalMs', event.target.value)} />
                    </label>
                    <label className="field">
                      <span>默认命令</span>
                      <input className="text-input" value={runtimeDraft.dashboardDefaultCommand} onChange={(event) => updateRuntimeDraft('dashboardDefaultCommand', event.target.value)} />
                    </label>
                    <label className="field">
                      <span>终端 Shell</span>
                      <input className="text-input" value={runtimeDraft.terminalShell} onChange={(event) => updateRuntimeDraft('terminalShell', event.target.value)} />
                    </label>
                    <label className="field">
                      <span>Agent HTTP 监听</span>
                      <input className="text-input" value={runtimeDraft.agentListenAddress} onChange={(event) => updateRuntimeDraft('agentListenAddress', event.target.value)} />
                    </label>
                    <label className="field">
                      <span>Agent gRPC 监听</span>
                      <input className="text-input" value={runtimeDraft.agentGrpcListenAddress} onChange={(event) => updateRuntimeDraft('agentGrpcListenAddress', event.target.value)} />
                    </label>
                    <label className="field">
                      <span>Agent 上报周期(秒)</span>
                      <input className="text-input" value={runtimeDraft.agentReportIntervalSeconds} onChange={(event) => updateRuntimeDraft('agentReportIntervalSeconds', event.target.value)} />
                    </label>
                    <label className="field">
                      <span>管理员用户名</span>
                      <input className="text-input" value={runtimeDraft.adminUser} onChange={(event) => updateRuntimeDraft('adminUser', event.target.value)} />
                    </label>
                    <label className="field">
                      <span>管理员新密码</span>
                      <input className="text-input" type="password" value={runtimeDraft.adminPassword} onChange={(event) => updateRuntimeDraft('adminPassword', event.target.value)} placeholder="留空则不修改" />
                    </label>
                  </div>
                  <div className="hero__actions">
                    <button className="button button--primary" onClick={saveRuntimeSettings} disabled={runtimeSaving}>
                      {runtimeSaving ? '保存中...' : '保存运行配置'}
                    </button>
                    <button
                      className="button"
                      onClick={() => {
                        setRuntimeDraft(toRuntimeDraft(runtimeSettings))
                        setRuntimeDirty(false)
                      }}
                      disabled={!runtimeDirty || runtimeSaving}
                    >
                      还原修改
                    </button>
                  </div>
                </div>
              ) : null}

              {dnsmasqSettings ? (
                <div className="control-stack">
                  <div className="panel-subtitle">dnsmasq / PXE 引导配置</div>
                  <div className="form-grid">
                    <label className="field">
                      <span>启用 PXE</span>
                      <label className="toggle">
                        <input
                          type="checkbox"
                          checked={dnsmasqDraft.enabled}
                          onChange={(event) => updateDnsmasqDraft('enabled', event.target.checked)}
                        />
                        <span>{dnsmasqDraft.enabled ? '已启用' : '未启用'}</span>
                      </label>
                    </label>
                    <label className="field">
                      <span>监听网卡</span>
                      <input className="text-input" value={dnsmasqDraft.listenInterface} onChange={(event) => updateDnsmasqDraft('listenInterface', event.target.value)} />
                    </label>
                    <label className="field">
                      <span>绑定地址</span>
                      <input className="text-input" value={dnsmasqDraft.bindAddress} onChange={(event) => updateDnsmasqDraft('bindAddress', event.target.value)} placeholder="可选，例如 192.168.56.1" />
                    </label>
                    <label className="field">
                      <span>下一跳服务器</span>
                      <input className="text-input" value={dnsmasqDraft.nextServer} onChange={(event) => updateDnsmasqDraft('nextServer', event.target.value)} />
                    </label>
                    <label className="field">
                      <span>DHCP 起始 IP</span>
                      <input className="text-input" value={dnsmasqDraft.dhcpRangeStart} onChange={(event) => updateDnsmasqDraft('dhcpRangeStart', event.target.value)} />
                    </label>
                    <label className="field">
                      <span>DHCP 结束 IP</span>
                      <input className="text-input" value={dnsmasqDraft.dhcpRangeEnd} onChange={(event) => updateDnsmasqDraft('dhcpRangeEnd', event.target.value)} />
                    </label>
                    <label className="field">
                      <span>租期</span>
                      <input className="text-input" value={dnsmasqDraft.dhcpLeaseTime} onChange={(event) => updateDnsmasqDraft('dhcpLeaseTime', event.target.value)} />
                    </label>
                    <label className="field">
                      <span>网关</span>
                      <input className="text-input" value={dnsmasqDraft.gateway} onChange={(event) => updateDnsmasqDraft('gateway', event.target.value)} />
                    </label>
                    <label className="field field--full">
                      <span>DNS 服务器</span>
                      <textarea value={dnsmasqDraft.dnsServersText} onChange={(event) => updateDnsmasqDraft('dnsServersText', event.target.value)} rows={2} placeholder="多个地址使用英文逗号或换行分隔" />
                    </label>
                    <label className="field field--full">
                      <span>TFTP 根目录</span>
                      <input className="text-input" value={dnsmasqDraft.tftpRoot} onChange={(event) => updateDnsmasqDraft('tftpRoot', event.target.value)} />
                    </label>
                    <label className="field">
                      <span>启动文件</span>
                      <input className="text-input" value={dnsmasqDraft.bootFile} onChange={(event) => updateDnsmasqDraft('bootFile', event.target.value)} />
                    </label>
                    <label className="field">
                      <span>PXE 菜单名称</span>
                      <input className="text-input" value={dnsmasqDraft.pxeServiceLabel} onChange={(event) => updateDnsmasqDraft('pxeServiceLabel', event.target.value)} />
                    </label>
                    <label className="field field--full">
                      <span>PXE 提示语</span>
                      <input className="text-input" value={dnsmasqDraft.pxePrompt} onChange={(event) => updateDnsmasqDraft('pxePrompt', event.target.value)} />
                    </label>
                    <label className="field">
                      <span>Kernel 路径</span>
                      <input className="text-input" value={dnsmasqDraft.kernelPath} onChange={(event) => updateDnsmasqDraft('kernelPath', event.target.value)} />
                    </label>
                    <label className="field">
                      <span>Initrd 路径</span>
                      <input className="text-input" value={dnsmasqDraft.initrdPath} onChange={(event) => updateDnsmasqDraft('initrdPath', event.target.value)} />
                    </label>
                    <label className="field field--full">
                      <span>内核启动参数</span>
                      <textarea value={dnsmasqDraft.bootArgs} onChange={(event) => updateDnsmasqDraft('bootArgs', event.target.value)} rows={3} />
                    </label>
                  </div>

                  <div className="hero__actions">
                    <button className="button button--primary" onClick={saveDnsmasqSettings} disabled={dnsmasqSaving}>
                      {dnsmasqSaving ? '保存中...' : '保存 dnsmasq 配置'}
                    </button>
                    <button
                      className="button"
                      onClick={() => {
                        setDnsmasqDraft(toDnsmasqDraft(dnsmasqSettings))
                        setDnsmasqDirty(false)
                      }}
                      disabled={!dnsmasqDirty || dnsmasqSaving}
                    >
                      还原未保存修改
                    </button>
                  </div>

                  <div className="overview-grid">
                    <div className="mini-stat"><span>配置文件</span><strong>{dnsmasqSettings.configPath}</strong></div>
                    <div className="mini-stat"><span>PXE 菜单</span><strong>{dnsmasqSettings.pxeConfigPath}</strong></div>
                    <div className="mini-stat"><span>最近更新时间</span><strong>{formatTime(dnsmasqSettings.updatedAt)}</strong></div>
                  </div>

                  <div className="dual-list">
                    <div>
                      <h3>dnsmasq.conf 预览</h3>
                      <pre>{dnsmasqSettings.renderedConfig}</pre>
                    </div>
                    <div>
                      <h3>PXE 菜单预览</h3>
                      <pre>{dnsmasqSettings.renderedPxeMenu}</pre>
                    </div>
                  </div>
                </div>
              ) : null}
            </Panel>
          )}

          {activeSection === '装机模板' && (
            <Panel title="装机模板" subtitle={selectedProfile?.name ?? '新模板'}>
              <div className="control-stack">
                <div className="target-list">
                  {installProfiles.map((profile) => (
                    <button
                      key={profile.id}
                      className={`target-chip target-chip--button${selectedProfileId === profile.id ? ' is-active' : ''}`}
                      onClick={() => {
                        setSelectedProfileId(profile.id)
                        setProfileDraft(toInstallProfileDraft(profile))
                        setProfileDirty(false)
                      }}
                    >
                      {profile.name}
                    </button>
                  ))}
                  <button
                    className="target-chip target-chip--button"
                    onClick={() => {
                      setSelectedProfileId('')
                      setProfileDraft(toInstallProfileDraft(null))
                      setProfileDirty(false)
                    }}
                  >
                    新建模板
                  </button>
                </div>

                <div className="form-grid">
                  <label className="field">
                    <span>模板名称</span>
                    <input className="text-input" value={profileDraft.name} onChange={(event) => updateProfileDraft('name', event.target.value)} />
                  </label>
                  <label className="field">
                    <span>发行版族</span>
                    <select className="text-input" value={profileDraft.osFamily} onChange={(event) => updateProfileDraft('osFamily', event.target.value)}>
                      <option value="ubuntu">ubuntu</option>
                      <option value="debian">debian</option>
                      <option value="fedora">fedora</option>
                      <option value="centos">centos</option>
                      <option value="rhel">rhel</option>
                    </select>
                  </label>
                  <label className="field">
                    <span>版本</span>
                    <input className="text-input" value={profileDraft.osVersion} onChange={(event) => updateProfileDraft('osVersion', event.target.value)} />
                  </label>
                  <label className="field">
                    <span>架构</span>
                    <input className="text-input" value={profileDraft.architecture} onChange={(event) => updateProfileDraft('architecture', event.target.value)} />
                  </label>
                  <label className="field">
                    <span>固件</span>
                    <input className="text-input" value={profileDraft.firmware} onChange={(event) => updateProfileDraft('firmware', event.target.value)} />
                  </label>
                  <label className="field">
                    <span>磁盘布局</span>
                    <input className="text-input" value={profileDraft.diskLayout} onChange={(event) => updateProfileDraft('diskLayout', event.target.value)} placeholder="Ubuntu: direct, EL: lvm" />
                  </label>
                  <label className="field field--full">
                    <span>安装源</span>
                    <input className="text-input" value={profileDraft.installSource} onChange={(event) => updateProfileDraft('installSource', event.target.value)} />
                  </label>
                  <label className="field">
                    <span>Kernel 路径</span>
                    <input className="text-input" value={profileDraft.bootKernelPath} onChange={(event) => updateProfileDraft('bootKernelPath', event.target.value)} />
                  </label>
                  <label className="field">
                    <span>Initrd 路径</span>
                    <input className="text-input" value={profileDraft.bootInitrdPath} onChange={(event) => updateProfileDraft('bootInitrdPath', event.target.value)} />
                  </label>
                  <label className="field">
                    <span>主机名模板</span>
                    <input className="text-input" value={profileDraft.hostnamePattern} onChange={(event) => updateProfileDraft('hostnamePattern', event.target.value)} placeholder="例如 ubuntu-${mac}" />
                  </label>
                  <label className="field">
                    <span>管理员用户名</span>
                    <input className="text-input" value={profileDraft.adminUsername} onChange={(event) => updateProfileDraft('adminUsername', event.target.value)} />
                  </label>
                  <label className="field field--full">
                    <span>管理员密码哈希</span>
                    <input className="text-input" value={profileDraft.adminPasswordHash} onChange={(event) => updateProfileDraft('adminPasswordHash', event.target.value)} placeholder="建议使用 openssl passwd -6 生成" />
                  </label>
                  <label className="field">
                    <span>时区</span>
                    <input className="text-input" value={profileDraft.timezone} onChange={(event) => updateProfileDraft('timezone', event.target.value)} />
                  </label>
                  <label className="field">
                    <span>语言环境</span>
                    <input className="text-input" value={profileDraft.locale} onChange={(event) => updateProfileDraft('locale', event.target.value)} />
                  </label>
                  <label className="field">
                    <span>键盘布局</span>
                    <input className="text-input" value={profileDraft.keyboardLayout} onChange={(event) => updateProfileDraft('keyboardLayout', event.target.value)} />
                  </label>
                  <label className="field">
                    <span>网络模式</span>
                    <input className="text-input" value={profileDraft.networkMode} onChange={(event) => updateProfileDraft('networkMode', event.target.value)} />
                  </label>
                  <label className="field field--full">
                    <span>SSH 公钥</span>
                    <textarea value={profileDraft.sshAuthorizedKeysText} onChange={(event) => updateProfileDraft('sshAuthorizedKeysText', event.target.value)} rows={4} />
                  </label>
                  <label className="field field--full">
                    <span>附加软件包</span>
                    <textarea value={profileDraft.packagesText} onChange={(event) => updateProfileDraft('packagesText', event.target.value)} rows={4} />
                  </label>
                  <label className="field field--full">
                    <span>Agent 二进制 URL</span>
                    <input className="text-input" value={profileDraft.agentBinaryUrl} onChange={(event) => updateProfileDraft('agentBinaryUrl', event.target.value)} placeholder="留空则使用 controller 暴露的 /provisioning/jobs/.../mxagent" />
                  </label>
                  <label className="field">
                    <span>Agent 服务名</span>
                    <input className="text-input" value={profileDraft.agentServiceName} onChange={(event) => updateProfileDraft('agentServiceName', event.target.value)} />
                  </label>
                  <label className="field">
                    <span>Controller gRPC 地址</span>
                    <input className="text-input" value={profileDraft.controllerGrpcAddress} onChange={(event) => updateProfileDraft('controllerGrpcAddress', event.target.value)} />
                  </label>
                  <label className="field field--full">
                    <span>额外内核参数</span>
                    <input className="text-input" value={profileDraft.extraKernelArgs} onChange={(event) => updateProfileDraft('extraKernelArgs', event.target.value)} />
                  </label>
                  <label className="field field--full">
                    <span>额外后置脚本</span>
                    <textarea value={profileDraft.postInstallScript} onChange={(event) => updateProfileDraft('postInstallScript', event.target.value)} rows={5} />
                  </label>
                  <label className="field">
                    <span>模板状态</span>
                    <label className="toggle">
                      <input type="checkbox" checked={profileDraft.enabled} onChange={(event) => updateProfileDraft('enabled', event.target.checked)} />
                      <span>{profileDraft.enabled ? '启用' : '停用'}</span>
                    </label>
                  </label>
                </div>

                <div className="hero__actions">
                  <button className="button button--primary" onClick={saveInstallProfile} disabled={profileSaving}>
                    {profileSaving ? '保存中...' : '保存装机模板'}
                  </button>
                  <button
                    className="button"
                    onClick={() => {
                      setProfileDraft(toInstallProfileDraft(selectedProfile))
                      setProfileDirty(false)
                    }}
                    disabled={!profileDirty || profileSaving}
                  >
                    还原修改
                  </button>
                </div>
              </div>
            </Panel>
          )}

          {activeSection === '装机任务' && (
            <Panel title="装机任务">
              <div className="control-stack">
                <div className="form-grid">
                  <label className="field">
                    <span>装机模板</span>
                    <select className="text-input" value={jobDraft.profileId} onChange={(event) => setJobDraft((current) => ({ ...current, profileId: event.target.value }))}>
                      {installProfiles.map((profile) => (
                        <option key={profile.id} value={profile.id}>{profile.name}</option>
                      ))}
                    </select>
                  </label>
                  <label className="field">
                    <span>MAC 地址</span>
                    <input className="text-input" value={jobDraft.macAddress} onChange={(event) => setJobDraft((current) => ({ ...current, macAddress: event.target.value }))} placeholder="例如 52:54:00:12:34:56" />
                  </label>
                  <label className="field">
                    <span>目标主机名</span>
                    <input className="text-input" value={jobDraft.hostname} onChange={(event) => setJobDraft((current) => ({ ...current, hostname: event.target.value }))} />
                  </label>
                  <label className="field">
                    <span>节点 ID</span>
                    <input className="text-input" value={jobDraft.nodeId} onChange={(event) => setJobDraft((current) => ({ ...current, nodeId: event.target.value }))} />
                  </label>
                </div>

                <div className="hero__actions">
                  <button className="button button--primary" onClick={createInstallJob} disabled={jobSubmitting}>
                    {jobSubmitting ? '创建中...' : '创建装机任务'}
                  </button>
                </div>

                {installJobs.map((job) => (
                  <div className="task-card" key={job.id}>
                    <div className="task-card__head">
                      <strong>{job.profileName}</strong>
                      <span className={`badge badge--${statusTone(job.status === 'failed' ? 'failed' : job.status === 'planned' ? 'pending' : 'running')}`}>{job.status}</span>
                      <span>{job.hostname}</span>
                      <span>{job.macAddress}</span>
                    </div>
                    <div className="overview-grid">
                      <div className="mini-stat"><span>Boot URL</span><strong>{job.bootUrl}</strong></div>
                      <div className="mini-stat"><span>配置 URL</span><strong>{job.configUrl}</strong></div>
                      <div className="mini-stat"><span>Agent 脚本</span><strong>{job.agentScriptUrl}</strong></div>
                    </div>
                    <div className="dual-list">
                      <div>
                        <h3>iPXE 预览</h3>
                        <pre>{job.bootPreview}</pre>
                      </div>
                      <div>
                        <h3>自动安装文件预览</h3>
                        <pre>{job.configPreview}</pre>
                      </div>
                    </div>
                  </div>
                ))}
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
