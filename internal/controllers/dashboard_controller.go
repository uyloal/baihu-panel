package controllers

import (
	"sort"
	"time"

	"github.com/uyloal/baihu-panel/internal/constant"
	"github.com/uyloal/baihu-panel/internal/database"
	"github.com/uyloal/baihu-panel/internal/models"
	"github.com/uyloal/baihu-panel/internal/services/tasks"
	"github.com/uyloal/baihu-panel/internal/utils"

	"github.com/gin-gonic/gin"
)

type DashboardController struct {
	executorService *tasks.ExecutorService
}

func NewDashboardController(executorService *tasks.ExecutorService) *DashboardController {
	return &DashboardController{
		executorService: executorService,
	}
}

type StatsResponse struct {
	Tasks      int64 `json:"tasks"`
	TodayExecs int64 `json:"today_execs"`
	Envs       int64 `json:"envs"`
	Logs       int64 `json:"logs"`
	Scheduled  int   `json:"scheduled"`
	Running    int   `json:"running"`
}

func (dc *DashboardController) GetStats(c *gin.Context) {
	var taskCount, envCount, logCount, todayExecs int64

	database.DB.Model(&models.Task{}).Count(&taskCount)
	database.DB.Model(&models.EnvironmentVariable{}).Count(&envCount)
	database.DB.Model(&models.TaskLog{}).Count(&logCount)

	// 今日执行总数
	today := time.Now().Format("2006-01-02")
	database.DB.Model(&models.SendStats{}).Where("day = ?", today).Select("COALESCE(SUM(num), 0)").Scan(&todayExecs)
	if todayExecs == 0 {
		database.DB.Model(&models.TaskLog{}).Where("created_at >= ?", today+" 00:00:00").Count(&todayExecs)
	}

	// 调度统计
	totalScheduled := dc.executorService.GetScheduledCount()

	// 正在运行：目前只能统计本地运行的任务
	// Agent 端的运行状态需要通过心跳上报（未来优化）
	running := dc.executorService.GetRunningCount()

	stats := StatsResponse{
		Tasks:      taskCount,
		TodayExecs: todayExecs,
		Envs:       envCount,
		Logs:       logCount,
		Scheduled:  totalScheduled,
		Running:    running,
	}

	utils.Success(c, stats)
}

// GetSentence 获取随机古诗词
func (dc *DashboardController) GetSentence(c *gin.Context) {
	utils.Success(c, gin.H{
		"sentence": constant.GetRandomSentence(),
	})
}

// DailyStats 每日统计数据
type DailyStats struct {
	Day     string `json:"day"`
	Total   int    `json:"total"`
	Success int    `json:"success"`
	Failed  int    `json:"failed"`
}

// GetSendStats 获取发送统计
func (dc *DashboardController) GetSendStats(c *gin.Context) {
	// 获取天数参数，默认30天
	days := 30
	if d := c.Query("days"); d != "" {
		if parsed, err := utils.ParseInt(d); err == nil && parsed > 0 && parsed <= 90 {
			days = parsed
		}
	}

	// 获取日期范围
	now := time.Now()
	startDay := now.AddDate(0, 0, -(days - 1)).Format("2006-01-02")

	var stats []models.SendStats
	database.DB.Where("day >= ?", startDay).Find(&stats)

	// 按日期聚合
	dayMap := make(map[string]*DailyStats)
	for _, s := range stats {
		if _, ok := dayMap[s.Day]; !ok {
			dayMap[s.Day] = &DailyStats{Day: s.Day}
		}
		ds := dayMap[s.Day]
		ds.Total += s.Num
		if s.Status == constant.TaskStatusSuccess {
			ds.Success += s.Num
		} else {
			ds.Failed += s.Num
		}
	}

	// 兜底：如果 send_stats 没有任何历史数据，从 task_logs 动态汇总
	if len(stats) == 0 {
		var logs []models.TaskLog
		database.DB.Where("created_at >= ?", startDay+" 00:00:00").Find(&logs)
		for _, l := range logs {
			if !l.CreatedAt.Time().IsZero() {
				d := l.CreatedAt.Time().Format("2006-01-02")
				if _, ok := dayMap[d]; !ok {
					dayMap[d] = &DailyStats{Day: d}
				}
				ds := dayMap[d]
				ds.Total++
				if l.Status == constant.TaskStatusSuccess {
					ds.Success++
				} else if l.Status == constant.TaskStatusFailed || l.Status == constant.TaskStatusTimeout {
					ds.Failed++
				}
			}
		}
	}

	// 填充缺失的日期
	result := make([]DailyStats, 0, days)
	for i := days - 1; i >= 0; i-- {
		day := now.AddDate(0, 0, -i).Format("2006-01-02")
		if ds, ok := dayMap[day]; ok {
			result = append(result, *ds)
		} else {
			result = append(result, DailyStats{Day: day})
		}
	}

	// 按日期排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].Day < result[j].Day
	})

	utils.Success(c, result)
}

// TaskStats 任务执行统计
type TaskStats struct {
	TaskID   string `json:"task_id"`
	TaskName string `json:"task_name"`
	Count    int    `json:"count"`
}

// GetTaskStats 获取任务执行占比
func (dc *DashboardController) GetTaskStats(c *gin.Context) {
	// 获取天数参数，默认30天
	days := 30
	if d := c.Query("days"); d != "" {
		if parsed, err := utils.ParseInt(d); err == nil && parsed > 0 && parsed <= 90 {
			days = parsed
		}
	}

	now := time.Now()
	startDay := now.AddDate(0, 0, -(days - 1)).Format("2006-01-02")

	// 按 task_id 聚合统计
	var results []struct {
		TaskID string
		Total  int
	}
	database.DB.Model(&models.SendStats{}).
		Select("task_id, SUM(num) as total").
		Where("day >= ?", startDay).
		Group("task_id").
		Order("total DESC").
		Find(&results)

	// 兜底：如果 send_stats 暂无数据，从 task_logs 聚合
	if len(results) == 0 {
		var logResults []struct {
			TaskID string
			Total  int
		}
		database.DB.Model(&models.TaskLog{}).
			Select("task_id, COUNT(*) as total").
			Where("created_at >= ?", startDay+" 00:00:00").
			Group("task_id").
			Order("total DESC").
			Find(&logResults)

		for _, lr := range logResults {
			if lr.TaskID != "" {
				results = append(results, struct {
					TaskID string
					Total  int
				}{
					TaskID: lr.TaskID,
					Total:  lr.Total,
				})
			}
		}
	}

	// 获取任务名称
	taskIDs := make([]string, 0, len(results))
	for _, r := range results {
		taskIDs = append(taskIDs, r.TaskID)
	}

	var tasks []models.Task
	if len(taskIDs) > 0 {
		database.DB.Where("id IN ?", taskIDs).Find(&tasks)
	}
	taskNameMap := make(map[string]string)
	for _, t := range tasks {
		taskNameMap[t.ID] = t.Name
	}

	// 构建结果
	stats := make([]TaskStats, 0, len(results))
	for _, r := range results {
		name := taskNameMap[r.TaskID]
		if name == "" {
			name = "未知任务"
		}
		stats = append(stats, TaskStats{
			TaskID:   r.TaskID,
			TaskName: name,
			Count:    r.Total,
		})
	}

	utils.Success(c, stats)
}
