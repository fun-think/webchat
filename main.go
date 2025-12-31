package main

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/olahol/melody"
)

//go:embed index.html
var index embed.FS

type Message struct {
	Type     string `json:"type"`
	Content  string `json:"content,omitempty"`
	UserID   string `json:"userId,omitempty"`
	UserName string `json:"userName,omitempty"`
}

type FileMessage struct {
	Type     string `json:"type"`
	Filename string `json:"filename"`
	Filesize int64  `json:"filesize"`
	UserID   string `json:"userId"`
	UserName string `json:"userName"`
}

type UploadResponse struct {
	Success  bool   `json:"success"`
	Filename string `json:"filename,omitempty"`
	Error    string `json:"error,omitempty"`
}

const (
	uploadDir      = "./files"
	maxUploadBytes = 20 << 20 // 20MiB
)

// 判断文件是否为图片
func isImageFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	imageExts := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".gif":  true,
		".bmp":  true,
		".webp": true,
	}
	return imageExts[ext]
}

func sanitizeFilename(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("文件名为空")
	}
	if strings.Contains(name, "\x00") {
		return "", errors.New("文件名非法")
	}
	if strings.Contains(name, "..") {
		return "", errors.New("禁止使用 ..")
	}
	if strings.ContainsAny(name, `/\`) {
		return "", errors.New("禁止使用路径分隔符")
	}

	base := filepath.Base(name)
	if base != name {
		return "", errors.New("文件名非法")
	}
	return base, nil
}

func dedupeFilename(dir, filename string) (string, error) {
	ext := filepath.Ext(filename)
	stem := strings.TrimSuffix(filename, ext)
	if stem == "" {
		stem = "file"
	}

	if _, err := os.Stat(filepath.Join(dir, filename)); err != nil {
		if os.IsNotExist(err) {
			return filename, nil
		}
		return "", err
	}

	for i := 1; i <= 9999; i++ {
		candidate := fmt.Sprintf("%s(%d)%s", stem, i, ext)
		if _, err := os.Stat(filepath.Join(dir, candidate)); err != nil {
			if os.IsNotExist(err) {
				return candidate, nil
			}
			return "", err
		}
	}
	return "", errors.New("同名文件过多")
}

func isSameOrigin(req *http.Request) bool {
	origin := req.Header.Get("Origin")
	if origin == "" {
		return true
	}
	originURL, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if originURL.Scheme != "http" && originURL.Scheme != "https" {
		return false
	}
	return originURL.Host == req.Host
}

func main() {
	r := gin.Default()
	r.MaxMultipartMemory = maxUploadBytes
	m := melody.New()
	m.Upgrader.CheckOrigin = isSameOrigin

	// 配置melody以支持更大的消息和更长的连接
	m.Config.MaxMessageSize = 1024 * 1024 // 1MB
	m.Config.MessageBufferSize = 1024
	m.Config.WriteWait = 10 * time.Second  // 写入超时时间
	m.Config.PongWait = 60 * time.Second   // 等待pong响应的时间
	m.Config.PingPeriod = 54 * time.Second // ping间隔（略小于PongWait）

	m.HandleConnect(func(s *melody.Session) {
		userName := "Unknown"
		if v, ok := s.Get("userName"); ok {
			if str, ok := v.(string); ok && strings.TrimSpace(str) != "" {
				userName = str
			}
		}
		msgBytes, _ := json.Marshal(Message{
			Type:    "system",
			Content: fmt.Sprintf("%s 加入聊天室", userName),
		})
		m.Broadcast(msgBytes)
	})

	// 确保files目录存在
	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		panic(err)
	}

	r.GET("/", func(c *gin.Context) {
		html, err := index.ReadFile("index.html")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "无法读取页面文件"})
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", html)
	})

	r.GET("/ws", func(c *gin.Context) {
		if !isSameOrigin(c.Request) {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		userID := strings.TrimSpace(c.Query("userId"))
		if userID == "" {
			userID = "Unknown"
		}
		userName := strings.TrimSpace(c.Query("userName"))
		if userName == "" {
			userName = "Unknown"
		}
		if err := m.HandleRequestWithKeys(c.Writer, c.Request, map[string]any{
			"userId":   userID,
			"userName": userName,
		}); err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
	})

	// 处理文件上传
	r.POST("/upload", func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadBytes)
		file, err := c.FormFile("file")
		if err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				c.JSON(http.StatusRequestEntityTooLarge, UploadResponse{
					Success: false,
					Error:   "文件过大",
				})
				return
			}
			if errors.Is(err, http.ErrMissingFile) {
				c.JSON(http.StatusBadRequest, UploadResponse{
					Success: false,
					Error:   "无法获取上传的文件",
				})
				return
			}
			if strings.Contains(strings.ToLower(err.Error()), "request body too large") {
				c.JSON(http.StatusRequestEntityTooLarge, UploadResponse{
					Success: false,
					Error:   "文件过大",
				})
				return
			}
			c.JSON(http.StatusBadRequest, UploadResponse{
				Success: false,
				Error:   "无法获取上传的文件",
			})
			return
		}

		userID := c.PostForm("userId")
		if userID == "" {
			userID = "Unknown"
		}
		userName := c.PostForm("userName")
		if userName == "" {
			userName = "Unknown"
		}

		originalName, err := sanitizeFilename(file.Filename)
		if err != nil {
			c.JSON(http.StatusBadRequest, UploadResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}

		safeName, err := dedupeFilename(uploadDir, originalName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, UploadResponse{
				Success: false,
				Error:   "无法生成文件名",
			})
			return
		}

		// 保存文件
		if err := c.SaveUploadedFile(file, filepath.Join(uploadDir, safeName)); err != nil {
			c.JSON(http.StatusInternalServerError, UploadResponse{
				Success: false,
				Error:   "无法保存文件",
			})
			return
		}

		// 通过WebSocket广播文件消息给所有用户
		fileMsg := FileMessage{
			Type:     "file",
			Filename: safeName,
			Filesize: file.Size,
			UserID:   userID,
			UserName: userName,
		}

		msgBytes, _ := json.Marshal(fileMsg)
		m.Broadcast(msgBytes)

		c.JSON(http.StatusOK, UploadResponse{
			Success:  true,
			Filename: safeName,
		})
	})

	// 提供文件下载
	r.GET("/files/:filename", func(c *gin.Context) {
		requestedName, err := sanitizeFilename(c.Param("filename"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		filePath := filepath.Join(uploadDir, requestedName)

		// 检查文件是否存在
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
			return
		}

		c.Header("X-Content-Type-Options", "nosniff")

		// 根据文件类型设置Content-Type
		if isImageFile(requestedName) {
			ext := strings.ToLower(filepath.Ext(requestedName))
			contentType := mime.TypeByExtension(ext)
			if contentType == "" {
				contentType = "application/octet-stream"
			}
			c.Header("Content-Type", contentType)
			c.File(filePath)
			return
		}

		c.FileAttachment(filePath, requestedName)
	})

	m.HandleMessage(func(s *melody.Session, msg []byte) {
		var message Message
		err := json.Unmarshal(msg, &message)
		if err != nil {
			// 如果不是JSON格式，当作普通文本处理
			m.Broadcast(msg)
			return
		}

		if message.Type == "text" {
			if strings.TrimSpace(message.UserID) == "" {
				message.UserID = "Unknown"
			}
			if strings.TrimSpace(message.UserName) == "" {
				message.UserName = "Unknown"
			}
			// 广播文本消息
			userMessage := Message{
				Type:     "user",
				UserID:   message.UserID,
				UserName: message.UserName,
				Content:  message.Content,
			}
			msgBytes, _ := json.Marshal(userMessage)
			m.Broadcast(msgBytes)
		} else if message.Type == "ping" {
			// 心跳消息，不需要处理
			return
		}
	})

	// 处理会话错误
	m.HandleError(func(s *melody.Session, err error) {
		log.Printf("websocket error: %v", err)
	})

	if err := r.Run(":5000"); err != nil {
		panic(err)
	}
}
