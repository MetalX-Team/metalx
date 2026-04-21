package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/gin-gonic/gin"
)

type Manager struct {
	store *Store
}

func New(databasePath, user, password string) (*Manager, error) {
	store, err := NewStore(databasePath)
	if err != nil {
		return nil, err
	}
	if err := store.EnsureAdmin(user, hashValue(password)); err != nil {
		return nil, err
	}
	return &Manager{store: store}, nil
}

func (m *Manager) Login(user, password string) (string, bool) {
	storedHash, ok := m.store.GetPasswordHash(user)
	if !ok || hashValue(password) != storedHash {
		return "", false
	}
	token := hashValue(user + password + time.Now().UTC().String())
	_ = m.store.SaveSession(token, user, time.Now().UTC().Add(12*time.Hour))
	return token, true
}

func (m *Manager) GinMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token == "" {
			token = c.Query("token")
		}
		if token == "" {
			c.AbortWithStatusJSON(401, gin.H{"error": "缺少鉴权令牌"})
			return
		}
		ok, err := m.store.ValidateSession(token)
		if err != nil {
			c.AbortWithStatusJSON(500, gin.H{"error": "会话校验失败"})
			return
		}
		if !ok {
			c.AbortWithStatusJSON(401, gin.H{"error": "令牌无效"})
			return
		}
		c.Next()
	}
}

func (m *Manager) Close() error {
	if m == nil {
		return nil
	}
	return m.store.Close()
}

func hashValue(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
