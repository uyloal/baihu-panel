package tasks

import (
	"encoding/json"
	"time"

	"github.com/uyloal/baihu-panel/internal/constant"
	"github.com/uyloal/baihu-panel/internal/database"
	"github.com/uyloal/baihu-panel/internal/logger"
	"github.com/uyloal/baihu-panel/internal/models"
	"github.com/uyloal/baihu-panel/internal/systime"
	"github.com/uyloal/baihu-panel/internal/utils"
)

// SendStatsService 接口定义（避免循环依赖）
type SendStatsService interface {
	IncrementStats(taskID string, status string) error
}

// TaskLogService 任务日志服务
type TaskLogService struct {
	sendStatsService SendStatsService
}

// NewTaskLogService 创建任务日志服务
func NewTaskLogService(sendStatsService SendStatsService) *TaskLogService {
	return &TaskLogService{
		sendStatsService: sendStatsService,
	}
}

// CleanConfig 清理配置
type CleanConfig struct {
	Type string `json:"type"` // day 或 count
	Keep int    `json:"keep"` // 保留天数或条数
}

// CreateTaskLog 创建任务日志记录
func (s *TaskLogService) CreateTaskLog(taskLog *models.TaskLog) error {
	if taskLog.ID == "" {
		taskLog.ID = utils.GenerateID()
	}
	if taskLog.CreatedAt.Time().IsZero() {
		taskLog.CreatedAt = models.Now()
	}
	err := database.DB.Create(taskLog).Error
	if err == nil && taskLog.StartTime != nil {
		database.DB.Model(&models.Task{}).Where("id = ?", taskLog.TaskID).Update("last_run", taskLog.StartTime)
	}
	return err
}

// GetTaskLogByID 根据 ID 获取任务日志
func (s *TaskLogService) GetTaskLogByID(id string) (*models.TaskLog, error) {
	var taskLog models.TaskLog
	err := database.DB.Where("id = ?", id).First(&taskLog).Error
	if err != nil {
		return nil, err
	}
	return &taskLog, nil
}

// UpdateTaskLog 更新任务日志
func (s *TaskLogService) UpdateTaskLog(taskLog *models.TaskLog) error {
	return database.DB.Model(&models.TaskLog{}).Where("id = ?", taskLog.ID).Updates(taskLog).Error
}

// UpdateTaskStatus 更新任务日志状态
func (s *TaskLogService) UpdateTaskStatus(logID string, status string) error {
	return database.DB.Model(&models.TaskLog{}).Where("id = ?", logID).Update("status", status).Error
}

// UpdateTaskDuration 更新任务耗时（心跳）
func (s *TaskLogService) UpdateTaskDuration(logID string, duration int64) error {
	return database.DB.Model(&models.TaskLog{}).Where("id = ?", logID).Update("duration", duration).Error
}

// UpdateLogCommand 更新日志中的命令内容
func (s *TaskLogService) UpdateLogCommand(logID string, command string) error {
	return database.DB.Model(&models.TaskLog{}).Where("id = ?", logID).Update("command", models.BigText(command)).Error
}

// UpdateTaskStats 更新任务统计
func (s *TaskLogService) UpdateTaskStats(taskID string, status string) {
	if s.sendStatsService == nil {
		logger.Error("[TaskLog] SendStatsService 未初始化")
		return
	}
	err := s.sendStatsService.IncrementStats(taskID, status)
	if err != nil {
		logger.Errorf("UpdateTaskStats err: %v", err)
	}
}

// CleanLogsByDays 按天数清理历史日志
func (s *TaskLogService) CleanLogsByDays(taskID string, days int) error {
	if days <= 0 {
		return nil
	}
	cutoff := systime.InCST(time.Now()).AddDate(0, 0, -days)
	return database.DB.Where("task_id = ? AND created_at < ?", taskID, cutoff).Delete(&models.TaskLog{}).Error
}

// CleanLogsByCount 按条数清理历史日志
func (s *TaskLogService) CleanLogsByCount(taskID string, keep int) error {
	if keep <= 0 {
		return nil
	}
	var boundaryLog models.TaskLog
	res := database.DB.Where("task_id = ?", taskID).Order("id DESC").Offset(keep - 1).Limit(1).Find(&boundaryLog)
	if res.Error == nil && res.RowsAffected > 0 {
		return database.DB.Where("task_id = ? AND id < ?", taskID, boundaryLog.ID).Delete(&models.TaskLog{}).Error
	}
	return nil
}

// CleanTaskLogs 清理任务日志
func (s *TaskLogService) CleanTaskLogs(taskID string) {
	var task models.Task
	res := database.DB.Where("id = ?", taskID).Limit(1).Find(&task)
	if res.Error != nil || res.RowsAffected == 0 {
		return
	}

	if task.CleanConfig == "" {
		return
	}

	var config CleanConfig
	if err := json.Unmarshal([]byte(task.CleanConfig), &config); err != nil {
		logger.Errorf("[TaskLog] 解析清理配置失败: %v", err)
		return
	}

	if config.Keep <= 0 {
		return
	}

	var deleted int64
	switch config.Type {
	case "day":
		cutoff := systime.InCST(time.Now()).AddDate(0, 0, -config.Keep)
		result := database.DB.Where("task_id = ? AND created_at < ?", taskID, cutoff).Delete(&models.TaskLog{})
		deleted = result.RowsAffected
	case "count":
		var boundaryLog models.TaskLog
		res := database.DB.Where("task_id = ?", taskID).Order("id DESC").Offset(config.Keep - 1).Limit(1).Find(&boundaryLog)
		if res.Error == nil && res.RowsAffected > 0 {
			result := database.DB.Where("task_id = ? AND id < ?", taskID, boundaryLog.ID).Delete(&models.TaskLog{})
			deleted = result.RowsAffected
		}
	}

	if deleted > 0 {
		logger.Infof("[TaskLog] 清理旧日志: #%s 共 %d 条", taskID, deleted)
	}
}

// ProcessTaskCompletion 处理任务完成后的所有操作
func (s *TaskLogService) ProcessTaskCompletion(taskLog *models.TaskLog) error {
	if err := s.UpdateTaskLog(taskLog); err != nil {
		return err
	}
	s.UpdateTaskStats(taskLog.TaskID, taskLog.Status)
	go s.CleanTaskLogs(taskLog.TaskID)
	return nil
}

// CreateTaskLogFromLocalExecution 从本地执行结果创建任务日志
func (s *TaskLogService) CreateTaskLogFromLocalExecution(taskID string, command, output, systemErr, status string, duration int64, exitCode int, start, end time.Time, isCompressed bool) (*models.TaskLog, error) {
	var compressed string
	var err error

	if isCompressed {
		compressed = output
	} else {
		trimmedOutput := utils.TrimLog(output, constant.MaxLogSize)
		compressed, err = utils.CompressToBase64(trimmedOutput)
		if err != nil {
			logger.Errorf("[TaskLog] 压缩日志失败: %v", err)
			compressed = ""
		}
	}

	startTime := models.LocalTime(start)
	endTime := models.LocalTime(end)

	taskLog := &models.TaskLog{
		ID:        utils.GenerateID(),
		TaskID:    taskID,
		Command:   models.BigText(command),
		Output:    models.BigText(compressed),
		Error:     models.BigText(systemErr),
		Status:    status,
		Duration:  duration,
		ExitCode:  exitCode,
		StartTime: &startTime,
		EndTime:   &endTime,
		CreatedAt: models.Now(),
	}

	return taskLog, nil
}
