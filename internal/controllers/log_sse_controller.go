package controllers

import (
	"fmt"
	"io"
	"time"

	"github.com/uyloal/baihu-panel/internal/constant"
	"github.com/uyloal/baihu-panel/internal/database"
	"github.com/uyloal/baihu-panel/internal/models"
	"github.com/uyloal/baihu-panel/internal/services/tasks"
	"github.com/uyloal/baihu-panel/internal/utils"

	"github.com/gin-gonic/gin"
)

type LogSSEController struct{}

func NewLogSSEController() *LogSSEController {
	return &LogSSEController{}
}

func (lc *LogSSEController) StreamLog(c *gin.Context) {
	logIDStr := c.Query("log_id")
	if logIDStr == "" {
		c.JSON(400, gin.H{"error": "log_id is required"})
		return
	}

	logID := logIDStr

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")
	c.Header("X-Accel-Buffering", "no")

	// 1. 检查数据库中日志记录状态
	var taskLog models.TaskLog
	res := database.DB.Where("id = ?", logID).Limit(1).Find(&taskLog)
	if res.Error == nil && res.RowsAffected > 0 {
		status := taskLog.Status
		// 只有明确已完成的状态才直接返回 finish 帧
		if status != constant.TaskStatusRunning && status != constant.TaskStatusPending {
			content, err := utils.DecompressFromBase64(string(taskLog.Output))
			if err != nil {
				content = "解压日志失败: " + err.Error()
			}
			endTimeStr := ""
			if taskLog.EndTime != nil {
				endTimeStr = taskLog.EndTime.Time().Format("2006-01-02 15:04:05")
			}
			c.SSEvent("message", gin.H{
				"type":      "finish",
				"text":      content,
				"status":    taskLog.Status,
				"duration":  taskLog.Duration,
				"end_time":  endTimeStr,
				"exit_code": taskLog.ExitCode,
			})
			c.Writer.Flush()
			return
		}
	}

	// 2. 任务处于 pending/running，轮询等待 TinyLog 就绪（最多等 30s）
	var tl *tasks.TinyLog
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		// 优先检查 context 是否已取消（客户端断开连接）
		select {
		case <-c.Request.Context().Done():
			return
		default:
		}

		tl = tasks.GetActiveLog(logID)
		if tl != nil {
			break
		}

		// TinyLog 还未就绪，检查数据库状态：
		// 如果任务已完成（非 pending/running），直接返回 finish 帧
		var checkLog models.TaskLog
		checkRes := database.DB.Where("id = ?", logID).Limit(1).Find(&checkLog)
		if checkRes.Error == nil && checkRes.RowsAffected > 0 {
			s := checkLog.Status
			if s != constant.TaskStatusRunning && s != constant.TaskStatusPending {
				content, err := utils.DecompressFromBase64(string(checkLog.Output))
				if err != nil {
					content = "解压日志失败: " + err.Error()
				}
				endTimeStr := ""
				if checkLog.EndTime != nil {
					endTimeStr = checkLog.EndTime.Time().Format("2006-01-02 15:04:05")
				}
				c.SSEvent("message", gin.H{
					"type":      "finish",
					"text":      content,
					"status":    checkLog.Status,
					"duration":  checkLog.Duration,
					"end_time":  endTimeStr,
					"exit_code": checkLog.ExitCode,
				})
				c.Writer.Flush()
				return
			}
		}

		time.Sleep(200 * time.Millisecond)
	}

	// 超时仍未找到 TinyLog
	if tl == nil {
		c.SSEvent("message", gin.H{
			"type":   "finish",
			"text":   "未找到正在运行的任务日志（等待超时）",
			"status": "failed",
		})
		c.Writer.Flush()
		return
	}

	// 发送系统提示
	c.SSEvent("message", gin.H{
		"type": "log",
		"text": fmt.Sprintf("[System] 连接成功，正在监听日志... (LogID: %s)\n", logID),
	})
	c.Writer.Flush()

	// 发送已缓冲的最后 100 行
	lastLines, err := tl.ReadLastLines(100)
	if err == nil && len(lastLines) > 0 {
		c.SSEvent("message", gin.H{
			"type": "log",
			"text": string(lastLines),
		})
		c.Writer.Flush()
	}

	// 订阅实时更新
	sub := tl.Subscribe()
	defer tl.Unsubscribe(sub)

	// 推送更新
	c.Stream(func(w io.Writer) bool {
		select {
		case data, ok := <-sub:
			if !ok {
				// 任务结束，读取库内落库后的最终真实数据，下发统一的 finish 帧
				var finalLog models.TaskLog
				status := constant.TaskStatusSuccess
				var duration int64
				endTimeStr := "-"
				var exitCode int

				// 防御性轮询等待数据库中的最终状态落库（最多 10 次，共 500ms）
				for i := 0; i < 10; i++ {
					dbRes := database.DB.Where("id = ?", logID).Limit(1).Find(&finalLog)
					if dbRes.Error == nil && dbRes.RowsAffected > 0 {
						if finalLog.Status != "" && finalLog.Status != constant.TaskStatusRunning && finalLog.Status != constant.TaskStatusPending {
							status = finalLog.Status
							duration = finalLog.Duration
							if finalLog.EndTime != nil {
								endTimeStr = finalLog.EndTime.Time().Format("2006-01-02 15:04:05")
							}
							exitCode = finalLog.ExitCode
							break
						}
					}
					time.Sleep(50 * time.Millisecond)
				}

				// 如果最终查询依然未获取到终态（如仍为 running/pending），强制根据 ExitCode/Error 兜底修正，严禁下发 status: running 的 finish 帧
				if status == constant.TaskStatusRunning || status == constant.TaskStatusPending {
					if finalLog.ExitCode != 0 || string(finalLog.Error) != "" {
						status = constant.TaskStatusFailed
					} else {
						status = constant.TaskStatusSuccess
					}
					if finalLog.Duration > 0 {
						duration = finalLog.Duration
					}
					if finalLog.EndTime != nil {
						endTimeStr = finalLog.EndTime.Time().Format("2006-01-02 15:04:05")
					} else {
						endTimeStr = time.Now().Format("2006-01-02 15:04:05")
					}
					exitCode = finalLog.ExitCode
				}

				c.SSEvent("message", gin.H{
					"type":      "finish",
					"text":      "\n--- 任务已结束 ---\n",
					"status":    status,
					"duration":  duration,
					"end_time":  endTimeStr,
					"exit_code": exitCode,
				})
				c.Writer.Flush()
				return false
			}
			c.SSEvent("message", gin.H{
				"type": "log",
				"text": string(data),
			})
			c.Writer.Flush()
			return true
		case <-c.Request.Context().Done():
			return false
		}
	})
}
