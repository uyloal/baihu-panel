package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/uyloal/baihu-panel/internal/constant"
	"github.com/uyloal/baihu-panel/internal/database"
	"github.com/uyloal/baihu-panel/internal/eventbus"
	"github.com/uyloal/baihu-panel/internal/executor"
	"github.com/uyloal/baihu-panel/internal/logger"
	"github.com/uyloal/baihu-panel/internal/models"
	"github.com/uyloal/baihu-panel/internal/utils"

	"gorm.io/gorm"
)

// SettingsService 接口定义（避免循环依赖）
type SettingsService interface {
	Get(section, key string) string
}

// EnvService 接口定义（避免循环依赖）
type EnvService interface {
	GetEnvVarsByIDs(ids string) []string
	GetAllEnvVars() []string
	GetEnvVarsAndSecretsByIDs(ids string) ([]string, []string)
	GetAllEnvVarsAndSecrets() ([]string, []string)
}

type ExecutorService struct {
	taskService     *TaskService
	taskLogService  *TaskLogService
	settingsService SettingsService
	envService      EnvService
	scheduler       *executor.Scheduler
	cronManager     *executor.CronManager
	results         []executor.ExecutionResult
	mu              sync.RWMutex
	resultsMu       sync.RWMutex
	stopCh          chan struct{}
}

func (es *ExecutorService) GetScheduler() *executor.Scheduler {
	return es.scheduler
}

func NewExecutorService(
	taskService *TaskService,
	taskLogService *TaskLogService,
	settingsService SettingsService,
	envService EnvService,
) *ExecutorService {
	// 0. 清理旧临时日志
	CleanupOrphanedTinyLogs()

	es := &ExecutorService{
		taskService:     taskService,
		taskLogService:  taskLogService,
		settingsService: settingsService,
		envService:      envService,
		results:         make([]executor.ExecutionResult, 0, 100),
		stopCh:          make(chan struct{}),
	}

	// 1. 初始化调度器
	es.initScheduler()

	// 2. 初始化计划任务管理器
	es.cronManager = executor.NewCronManager(es.scheduler)
	es.cronManager.OnTrigger = func(t executor.CronTask) *executor.ExecutionRequest {
		task := es.taskService.GetTaskByID(t.GetID())
		return es.CreateExecutionRequest(task, executor.TaskTypeCron, nil)
	}

	return es
}

func (es *ExecutorService) initScheduler() {
	workerCount := getIntSetting(es.settingsService, constant.SectionScheduler, constant.KeyWorkerCount, 4)
	queueSize := getIntSetting(es.settingsService, constant.SectionScheduler, constant.KeyQueueSize, 100)
	rateInterval := getIntSetting(es.settingsService, constant.SectionScheduler, constant.KeyRateInterval, 200)

	cfg := executor.SchedulerConfig{
		WorkerCount:  workerCount,
		QueueSize:    queueSize,
		RateInterval: time.Duration(rateInterval) * time.Millisecond,
		StrictQueue:  false,
		Verbose:      false,
	}

	handler := NewSchedulerHandler(es)
	es.scheduler = executor.NewScheduler(cfg, handler)
	es.scheduler.SetExecutor(es.ExecuteDispatcher)
	es.scheduler.Start()
}

// SchedulerHandler 实现 executor.SchedulerEventHandler 接口
type SchedulerHandler struct {
	es *ExecutorService
}

func NewSchedulerHandler(es *ExecutorService) *SchedulerHandler {
	return &SchedulerHandler{es: es}
}

// OnTaskScheduled 任务加入等待队列时触发
func (h *SchedulerHandler) OnTaskScheduled(req *executor.ExecutionRequest) {
	if req.TaskID != "" && req.Type != executor.TaskTypeSystem {
		_ = h.es.StartTaskExecutionRecord(req, constant.TaskStatusPending)
	}
}

// OnTaskExecuting 任务准备开始执行时触发，返回用于接收实时日志的 Writer
func (h *SchedulerHandler) OnTaskExecuting(req *executor.ExecutionRequest) (stdout, stderr io.Writer, err error) {
	if req.TaskID == "" || req.Type == executor.TaskTypeSystem {
		return nil, nil, nil
	}

	// 确保生成 LogID 并创建日志记录
	if req.LogID == "" {
		taskLog := h.es.StartTaskExecutionRecord(req, constant.TaskStatusRunning)
		req.LogID = taskLog.ID
	} else {
		// 如果在 Scheduled 阶段已经生成了 LogID，此时将其状态变更为 running
		h.es.taskLogService.UpdateTaskStatus(req.LogID, constant.TaskStatusRunning)
		startTimeStr := time.Now().Format("2006-01-02 15:04:05")
		eventbus.DefaultBus.Publish(eventbus.Event{
			Type: constant.EventTaskRunning,
			Payload: map[string]interface{}{
				"task_id":    req.TaskID,
				"task_name":  req.Name,
				"log_id":     req.LogID,
				"status":     constant.TaskStatusRunning,
				"start_time": startTimeStr,
			},
		})
	}

	masks := append([]string{}, req.Secrets...)
	masks = append(masks, utils.GetSystemSecrets()...)
	tinyLog, err := NewTinyLog(req.LogID, masks)
	if err != nil {
		return nil, nil, err
	}
	return tinyLog, tinyLog, nil
}

// OnTaskStarted 任务实际开始运行（获取到 worker 协程且通过速率限制后）触发
func (h *SchedulerHandler) OnTaskStarted(req *executor.ExecutionRequest) {
	if req.TaskID != "" && req.Type != executor.TaskTypeSystem {
		goid, err := h.es.AddRunningGo(req.TaskID)
		if err == nil {
			req.Metadata.GoID = goid
		}
	}
}

// OnTaskCompleted 任务执行完成时触发
func (h *SchedulerHandler) OnTaskCompleted(req *executor.ExecutionRequest, result *executor.ExecutionResult) {
	if req.TaskID != "" && req.Type != executor.TaskTypeSystem {
		if req.Metadata.GoID > 0 {
			h.es.RemoveRunningGo(req.TaskID, req.Metadata.GoID)
		}

		if req.LogID != "" {
			_ = h.es.FinishTaskExecutionRecord(req.LogID, result)
		}

		var startTimeStr, endTimeStr string
		if !result.StartTime.IsZero() {
			startTimeStr = result.StartTime.Format("2006-01-02 15:04:05")
		}
		if !result.EndTime.IsZero() {
			endTimeStr = result.EndTime.Format("2006-01-02 15:04:05")
		}

		eventPayload := map[string]interface{}{
			"task_id":    req.TaskID,
			"task_name":  req.Name,
			"log_id":     req.LogID,
			"status":     result.Status,
			"duration":   result.Duration,
			"exit_code":  result.ExitCode,
			"start_time": startTimeStr,
			"end_time":   endTimeStr,
			"output":     result.Output,
			"error":      result.Error,
		}

		// 发布任务完成事件到事件总线
		switch result.Status {
		case constant.TaskStatusSuccess:
			eventbus.DefaultBus.Publish(eventbus.Event{Type: constant.EventTaskSuccess, Payload: eventPayload})
		case constant.TaskStatusFailed:
			eventbus.DefaultBus.Publish(eventbus.Event{Type: constant.EventTaskFailed, Payload: eventPayload})
		case constant.TaskStatusTimeout:
			eventbus.DefaultBus.Publish(eventbus.Event{Type: constant.EventTaskTimeout, Payload: eventPayload})
		case constant.TaskStatusCancelled:
			eventbus.DefaultBus.Publish(eventbus.Event{Type: constant.EventTaskCancelled, Payload: eventPayload})
		}

		task := h.es.taskService.GetTaskByID(req.TaskID)
		if task != nil {
			h.es.cleanOldLogs(task)
		}
	}

	h.es.UpdateResult(*result)
}

// OnTaskFailed 任务调度失败触发
func (h *SchedulerHandler) OnTaskFailed(req *executor.ExecutionRequest, err error) {
	if req.TaskID != "" && req.Type != executor.TaskTypeSystem {
		if req.LogID == "" {
			taskLog := h.es.StartTaskExecutionRecord(req, constant.TaskStatusFailed)
			req.LogID = taskLog.ID
		}
		result := &executor.ExecutionResult{
			TaskID:    req.TaskID,
			LogID:     req.LogID,
			Success:   false,
			Status:    constant.TaskStatusFailed,
			Error:     err.Error(),
			StartTime: time.Now(),
			EndTime:   time.Now(),
		}
		_ = h.es.FinishTaskExecutionRecord(req.LogID, result)
		h.es.UpdateResult(*result)
	}
}

// OnCronNextRun 计划任务下次运行时间更新时触发
func (h *SchedulerHandler) OnCronNextRun(req *executor.ExecutionRequest, nextRun time.Time) {
	if req.TaskID != "" {
		nt := models.LocalTime(nextRun)
		database.DB.Model(&models.Task{}).Where("id = ?", req.TaskID).Update("next_run", nt)
	}
}

// OnTaskHeartbeat 任务执行心跳
func (h *SchedulerHandler) OnTaskHeartbeat(req *executor.ExecutionRequest, duration int64) {
	if req.LogID != "" {
		_ = h.es.taskLogService.UpdateTaskDuration(req.LogID, duration)
	}
}

// StartTaskExecutionRecord 开始记录任务执行日志
func (es *ExecutorService) StartTaskExecutionRecord(req *executor.ExecutionRequest, status string) *models.TaskLog {
	task := es.taskService.GetTaskByID(req.TaskID)
	if task == nil {
		return nil
	}

	startTime := models.Now()
	taskLog := &models.TaskLog{
		ID:        utils.GenerateID(),
		TaskID:    task.ID,
		Command:   models.BigText(req.MaskedCommand),
		Status:    status,
		StartTime: &startTime,
	}

	if req.LogID != "" {
		taskLog.ID = req.LogID
	} else {
		req.LogID = taskLog.ID
	}

	_ = es.taskLogService.CreateTaskLog(taskLog)

	startTimeStr := startTime.Time().Format("2006-01-02 15:04:05")
	if status == constant.TaskStatusRunning {
		eventbus.DefaultBus.Publish(eventbus.Event{
			Type: constant.EventTaskRunning,
			Payload: map[string]interface{}{
				"task_id":    task.ID,
				"task_name":  task.Name,
				"log_id":     taskLog.ID,
				"status":     constant.TaskStatusRunning,
				"start_time": startTimeStr,
			},
		})
	} else if status == constant.TaskStatusPending {
		eventbus.DefaultBus.Publish(eventbus.Event{
			Type: constant.EventTaskQueued,
			Payload: map[string]interface{}{
				"task_id":   task.ID,
				"task_name": task.Name,
				"log_id":    taskLog.ID,
				"status":    constant.TaskStatusPending,
			},
		})
	}

	return taskLog
}

// FinishTaskExecutionRecord 结束任务执行日志并更新数据库
func (es *ExecutorService) FinishTaskExecutionRecord(logID string, result *executor.ExecutionResult) error {
	taskLog, err := es.taskLogService.GetTaskLogByID(logID)
	if err != nil || taskLog == nil {
		return err
	}

	endTime := models.Now()
	taskLog.EndTime = &endTime
	taskLog.Duration = result.Duration
	taskLog.Status = result.Status
	taskLog.ExitCode = result.ExitCode

	if result.Error != "" {
		taskLog.Error = models.BigText(result.Error)
	}

	// 从 TinyLog 临时文件中读取完整日志，并使用 Gzip + Base64 压缩存储到数据库
	tinyLog := GetActiveLog(logID)
	if tinyLog != nil {
		compressed, compErr := tinyLog.CompressAndCleanup()
		if compErr == nil && compressed != "" {
			taskLog.Output = models.BigText(compressed)
		}
	} else if result.Output != "" {
		compressed, compErr := utils.CompressToBase64(result.Output)
		if compErr == nil {
			taskLog.Output = models.BigText(compressed)
		}
	}

	// 同步落库 TaskLog，确保依赖该记录的后续查询（如 SSE finish 帧）读取到最新状态
	_ = es.taskLogService.UpdateTaskLog(taskLog)
	es.taskLogService.UpdateTaskStats(taskLog.TaskID, taskLog.Status)

	// 异步更新任务元数据与统计
	go func() {
		task := es.taskService.GetTaskByID(taskLog.TaskID)
		if task != nil {
			task.LastRun = &endTime
			_ = es.taskService.UpdateTask(task.ID, &TaskParam{
				Name:          task.Name,
				Remark:        task.Remark,
				Command:       string(task.Command),
				PreCommand:    string(task.PreCommand),
				PostCommand:   string(task.PostCommand),
				Tags:          task.Tags,
				Type:          task.Type,
				Config:        string(task.Config),
				Schedule:      task.Schedule,
				Timeout:       task.Timeout,
				WorkDir:       task.WorkDir,
				CleanConfig:   task.CleanConfig,
				Envs:          string(task.Envs),
				TriggerType:   task.TriggerType,
				RetryCount:    task.RetryCount,
				RetryInterval: task.RetryInterval,
				RandomRange:   task.RandomRange,
				SourceID:      task.SourceID,
				PinType:       task.PinType,
				Enabled:       utils.DerefBool(task.Enabled, true),
			})
		}
	}()

	return nil
}

type LocalTaskHooks struct {
	es    *ExecutorService
	logID string
}

func (h *LocalTaskHooks) PreExecute(ctx context.Context, req executor.Request) (string, error) {
	return h.logID, nil
}

func (h *LocalTaskHooks) PostExecute(ctx context.Context, logID string, result *executor.Result) error {
	return nil
}

func (h *LocalTaskHooks) OnHeartbeat(ctx context.Context, logID string, duration int64) error {
	if logID != "" {
		return h.es.taskLogService.UpdateTaskDuration(logID, duration)
	}
	return nil
}

// ExecuteDispatcher 实现任务分发逻辑
func (es *ExecutorService) ExecuteDispatcher(ctx context.Context, req *executor.ExecutionRequest, stdout, stderr io.Writer) (*executor.Result, error) {
	taskID := req.TaskID

	req.Command = es.ResolvePath(req.Command)
	req.PreCommand = es.ResolvePath(req.PreCommand)
	req.PostCommand = es.ResolvePath(req.PostCommand)
	req.WorkDir = es.ResolvePath(req.WorkDir)

	task := es.taskService.GetTaskByID(taskID)
	if task == nil {
		return executor.Execute(ctx, executor.Request{
			Command:     req.Command,
			PreCommand:  req.PreCommand,
			PostCommand: req.PostCommand,
			WorkDir:     req.WorkDir,
			Envs:        req.Envs,
			Timeout:     req.Timeout,
		}, stdout, stderr)
	}

	es.refreshExecutionRequestEnvs(req, task)
	logger.Infof("[Executor] 任务最终执行命令: %s", req.MaskedCommand)

	hooks := &LocalTaskHooks{es: es, logID: req.LogID}
	return executor.ExecuteWithHooks(ctx, executor.Request{
		Command:     req.Command,
		PreCommand:  req.PreCommand,
		PostCommand: req.PostCommand,
		WorkDir:     req.WorkDir,
		Envs:        req.Envs,
		Timeout:     req.Timeout,
	}, stdout, stderr, hooks)
}

func getIntSetting(s SettingsService, section, key string, defaultVal int) int {
	val := s.Get(section, key)
	if val == "" {
		return defaultVal
	}
	var result int
	if _, err := fmt.Sscanf(val, "%d", &result); err != nil {
		return defaultVal
	}
	return result
}

func (es *ExecutorService) Stop() {
	es.StopCron()
	es.scheduler.Stop()
}

func (es *ExecutorService) Reload() {
	es.Stop()
	es.initScheduler()
	es.StartCron()
}

func (es *ExecutorService) StartCron() {
	go es.loadCronTasks()
	es.cronManager.Start()
}

func (es *ExecutorService) StopCron() {
	es.cronManager.Stop()
}

func (es *ExecutorService) AddCronTask(task *models.Task) error {
	if task.TriggerType != constant.TriggerTypeCron {
		es.RemoveCronTask(task.ID)
		return nil
	}
	envs, secrets := es.loadEnvVars(task.ID, string(task.Envs))
	task.RuntimeEnvs = envs
	task.RuntimeSecrets = secrets

	return es.cronManager.AddTask(task)
}

func (es *ExecutorService) RemoveCronTask(taskID string) {
	es.cronManager.RemoveTask(taskID)
}

func (es *ExecutorService) ValidateCron(schedule string) error {
	return es.cronManager.ValidateCron(schedule)
}

func (es *ExecutorService) loadCronTasks() {
	tasksList := es.taskService.GetTasks()
	for _, task := range tasksList {
		if task.TriggerType == constant.TriggerTypeBaihuStartup {
			go func(t models.Task) {
				req := es.CreateExecutionRequest(&t, executor.TaskTypeSystem, nil)
				es.scheduler.EnqueueOrExecute(req)
			}(task)
		} else if task.TriggerType == constant.TriggerTypeCron && task.Schedule != "" {
			es.cronManager.AddTask(&task)
		}
	}
}

func (es *ExecutorService) CreateExecutionRequest(task *models.Task, triggerType executor.TaskType, extraEnvs []string) *executor.ExecutionRequest {
	if task == nil {
		return nil
	}

	envs, secrets := es.loadEnvVars(task.ID, string(task.Envs))
	if len(extraEnvs) > 0 {
		envs = append(envs, extraEnvs...)
	}

	command := string(task.Command)
	preCommand := string(task.PreCommand)
	postCommand := string(task.PostCommand)
	workDir := task.WorkDir
	if workDir == "" {
		workDir = constant.ScriptsWorkDir
	}

	if task.Type == constant.TaskTypeRepo {
		repoCmd, repoWorkDir := es.BuildRepoCommand(task)
		if repoCmd != "" {
			command = repoCmd
			workDir = repoWorkDir
			preCommand = ""
			postCommand = ""

			var repoCfg models.RepoConfig
			if err := json.Unmarshal([]byte(task.Config), &repoCfg); err == nil && repoCfg.AuthToken != "" {
				secrets = append(secrets, repoCfg.AuthToken)
			}
		}
	}

	masks := append([]string{}, secrets...)
	masks = append(masks, utils.GetSystemSecrets()...)
	maskedCommand := utils.MaskSecrets(command, masks)

	return &executor.ExecutionRequest{
		TaskID:        task.ID,
		Name:          task.Name,
		Type:          triggerType,
		Command:       command,
		MaskedCommand: maskedCommand,
		PreCommand:    preCommand,
		PostCommand:   postCommand,
		WorkDir:       workDir,
		Envs:          envs,
		Secrets:       secrets,
		Timeout:       task.Timeout,
	}
}

func (es *ExecutorService) ExecuteTask(taskID string, extraEnvs []string) *executor.ExecutionResult {
	task := es.taskService.GetTaskByID(taskID)
	if task == nil {
		return &executor.ExecutionResult{
			TaskID:    taskID,
			Success:   false,
			Error:     "任务不存在",
			StartTime: time.Now(),
			EndTime:   time.Now(),
		}
	}

	if err := es.CheckConcurrency(taskID); err != nil {
		return &executor.ExecutionResult{
			TaskID:    taskID,
			Success:   false,
			Error:     err.Error(),
			StartTime: time.Now(),
			EndTime:   time.Now(),
		}
	}

	req := es.CreateExecutionRequest(task, executor.TaskTypeManual, extraEnvs)
	es.scheduler.EnqueueOrExecute(req)

	return &executor.ExecutionResult{
		TaskID:    taskID,
		LogID:     req.LogID,
		Success:   true,
		Status:    constant.TaskStatusPending,
		StartTime: time.Now(),
	}
}

func (es *ExecutorService) StopTask(logID string) error {
	taskLog, err := es.taskLogService.GetTaskLogByID(logID)
	if err != nil || taskLog == nil {
		return fmt.Errorf("停止失败：日志记录不存在")
	}

	if taskLog.Status != constant.TaskStatusRunning && taskLog.Status != constant.TaskStatusPending {
		statusText := taskLog.Status
		if taskLog.Status == constant.TaskStatusSuccess {
			statusText = "已完成"
		} else if taskLog.Status == constant.TaskStatusFailed {
			statusText = "已失败"
		}
		return fmt.Errorf("操作无效：任务当前状态为 [%s]，无需停止", statusText)
	}

	task := es.taskService.GetTaskByID(taskLog.TaskID)
	if task == nil {
		return fmt.Errorf("停止失败：关联的任务信息已丢失")
	}

	logger.Infof("[Executor] 请求停止本地任务 #%s (LogID: %s)", task.ID, logID)
	if es.scheduler.StopLog(logID) {
		return nil
	}

	taskLog.Status = constant.TaskStatusFailed
	errorMessage := "任务执行实例已丢失（可能由于系统重启导致），已自动同步状态为失败"
	taskLog.Error = models.BigText(errorMessage)

	database.DB.Model(&taskLog).Updates(map[string]interface{}{
		"status": taskLog.Status,
		"error":  taskLog.Error,
	})

	return fmt.Errorf("停止失败：%s", errorMessage)
}

func (es *ExecutorService) GetRunningCount() int {
	return es.scheduler.GetRunningTaskCount()
}

func (es *ExecutorService) GetScheduledCount() int {
	return es.cronManager.GetScheduledCount()
}

func (es *ExecutorService) ExecuteCommand(command string) *executor.ExecutionResult {
	return es.ExecuteCommandWithTimeout(command, time.Duration(constant.DefaultTaskTimeout)*time.Minute)
}

func (es *ExecutorService) ExecuteCommandWithTimeout(command string, timeout time.Duration) *executor.ExecutionResult {
	return es.ExecuteCommandWithEnv(command, timeout, nil)
}

func (es *ExecutorService) ExecuteCommandWithEnv(command string, timeout time.Duration, envVars []string) *executor.ExecutionResult {
	return es.ExecuteCommandWithOptions(command, timeout, envVars, "")
}

func (es *ExecutorService) ExecuteCommandWithOptions(command string, timeout time.Duration, envVars []string, workDir string) *executor.ExecutionResult {
	if workDir == "" {
		workDir = constant.ScriptsWorkDir
	}
	req := &executor.ExecutionRequest{
		Command: command,
		Timeout: int(timeout.Minutes()),
		Envs:    envVars,
		WorkDir: workDir,
		Type:    executor.TaskTypeSystem,
	}

	res, _ := es.scheduler.ExecuteSync(req)
	return res
}

func (es *ExecutorService) UpdateResult(res executor.ExecutionResult) {
	es.resultsMu.Lock()
	defer es.resultsMu.Unlock()

	isFinished := res.Status == constant.TaskStatusSuccess ||
		res.Status == constant.TaskStatusFailed ||
		res.Status == constant.TaskStatusTimeout ||
		res.Status == constant.TaskStatusCancelled

	if isFinished {
		res.Output = ""
	}

	for i := range es.results {
		if es.results[i].LogID == res.LogID && res.LogID != "" {
			es.results[i] = res
			return
		}
	}

	if len(es.results) >= 100 {
		es.results = es.results[1:]
	}
	es.results = append(es.results, res)
}

func (es *ExecutorService) GetLastResults(count int) []executor.ExecutionResult {
	es.resultsMu.RLock()
	defer es.resultsMu.RUnlock()

	total := len(es.results)
	if count > total {
		count = total
	}

	if count <= 0 {
		return []executor.ExecutionResult{}
	}

	res := make([]executor.ExecutionResult, 0, count)
	for i := 0; i < count; i++ {
		res = append(res, es.results[total-1-i])
	}
	return res
}

func (es *ExecutorService) CleanupRunningTasks() error {
	logger.Info("[Executor] 正在清理残留的任务运行状态...")
	return database.DB.Model(&models.Task{}).Where("1=1").Update("running_go", "[]").Error
}

func (es *ExecutorService) CheckConcurrency(taskID string) error {
	var task models.Task
	res := database.DB.Select("config, running_go").Where("id = ?", taskID).Limit(1).Find(&task)
	if res.Error != nil || res.RowsAffected == 0 {
		if res.Error != nil {
			return res.Error
		}
		return gorm.ErrRecordNotFound
	}
	var goids []int64
	if string(task.RunningGo) != "" {
		_ = json.Unmarshal([]byte(string(task.RunningGo)), &goids)
	}

	var config models.TaskConfig
	if string(task.Config) != "" {
		_ = json.Unmarshal([]byte(string(task.Config)), &config)
	}

	if config.Concurrency == 0 && len(goids) > 0 {
		return fmt.Errorf("任务正在运行中，拒绝并行执行，请前往日志查看")
	}
	return nil
}

func (es *ExecutorService) AddRunningGo(taskID string) (int64, error) {
	goid := utils.GetGoroutineID()
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		lastErr = database.DB.Transaction(func(tx *gorm.DB) error {
			var task models.Task
			res := tx.Where("id = ?", taskID).Limit(1).Find(&task)
			if res.Error != nil || res.RowsAffected == 0 {
				if res.Error != nil {
					return res.Error
				}
				return gorm.ErrRecordNotFound
			}
			var goids []int64
			if task.RunningGo != "" {
				_ = json.Unmarshal([]byte(task.RunningGo), &goids)
			}

			var config models.TaskConfig
			if task.Config != "" {
				_ = json.Unmarshal([]byte(task.Config), &config)
			}

			if config.Concurrency == 0 && len(goids) > 0 {
				return fmt.Errorf("task is running")
			}

			goids = append(goids, goid)
			data, _ := json.Marshal(goids)
			return tx.Model(&task).Update("running_go", models.BigText(data)).Error
		})
		if lastErr == nil {
			return goid, nil
		}
		if lastErr.Error() == "task is running" {
			return goid, lastErr
		}
		time.Sleep(100 * time.Millisecond)
	}
	return goid, fmt.Errorf("任务并发限制: %v", lastErr)
}

func (es *ExecutorService) RemoveRunningGo(taskID string, goid int64) {
	for attempt := 0; attempt < 3; attempt++ {
		err := database.DB.Transaction(func(tx *gorm.DB) error {
			var task models.Task
			res := tx.Where("id = ?", taskID).Limit(1).Find(&task)
			if res.Error != nil || res.RowsAffected == 0 {
				if res.Error != nil {
					return res.Error
				}
				return gorm.ErrRecordNotFound
			}
			var goids []int64
			if task.RunningGo != "" {
				_ = json.Unmarshal([]byte(task.RunningGo), &goids)
			}
			newGoids := make([]int64, 0)
			for _, id := range goids {
				if id != goid {
					newGoids = append(newGoids, id)
				}
			}
			data, _ := json.Marshal(newGoids)
			return tx.Model(&task).Update("running_go", string(data)).Error
		})
		if err == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (es *ExecutorService) cleanOldLogs(task *models.Task) {
	if task.CleanConfig == "" {
		return
	}

	var cleanCfg models.CleanConfig
	if err := json.Unmarshal([]byte(task.CleanConfig), &cleanCfg); err == nil {
		if cleanCfg.Type == "day" && cleanCfg.Keep > 0 {
			_ = es.taskLogService.CleanLogsByDays(task.ID, cleanCfg.Keep)
		} else if cleanCfg.Type == "count" && cleanCfg.Keep > 0 {
			_ = es.taskLogService.CleanLogsByCount(task.ID, cleanCfg.Keep)
		}
	}
}

func (es *ExecutorService) BuildRepoCommand(task *models.Task) (string, string) {
	exePath, err := os.Executable()
	if err != nil {
		exePath = "baihu"
	}

	var config models.RepoConfig
	if err := json.Unmarshal([]byte(task.Config), &config); err != nil {
		logger.Errorf("[Executor] 解析任务配置失败: %v", err)
		return "", ""
	}

	targetPath := config.TargetPath
	if targetPath == "" {
		targetPath = task.WorkDir
	}

	scriptsDir := utils.ResolveAbsScriptsDir()
	absTargetPath := targetPath
	if !filepath.IsAbs(targetPath) {
		absTargetPath = filepath.Join(scriptsDir, targetPath)
	}

	displayTargetPath := targetPath
	if rel, err := filepath.Rel(scriptsDir, absTargetPath); err == nil && !strings.HasPrefix(rel, "..") {
		if rel == "." {
			displayTargetPath = constant.ScriptsDirPlaceholder
		} else {
			displayTargetPath = constant.ScriptsDirPlaceholder + "/" + filepath.ToSlash(rel)
		}
	}

	args := []string{
		"reposync",
		"--source-type", config.SourceType,
		"--source-url", config.SourceURL,
		"--target-path", displayTargetPath,
	}
	if config.Branch != "" {
		args = append(args, "--branch", config.Branch)
	}
	if config.SparsePath != "" {
		args = append(args, "--path", config.SparsePath)
	}
	if config.SingleFile {
		args = append(args, "--single-file")
	}
	if config.Proxy != "" && config.Proxy != "none" {
		args = append(args, "--proxy", config.Proxy)
		if config.Proxy == "custom" && config.ProxyURL != "" {
			args = append(args, "--proxy-url", config.ProxyURL)
		}
	}
	if config.AuthToken != "" {
		args = append(args, "--auth-token", config.AuthToken)
	}
	if config.WhitelistPaths != "" {
		args = append(args, "--whitelist-paths", config.WhitelistPaths)
	}
	if config.Blacklist != "" {
		args = append(args, "--blacklist", config.Blacklist)
	}
	if config.Dependence != "" {
		args = append(args, "--dependence", config.Dependence)
	}
	if config.CommentToTask == "true" {
		args = append(args, "--commenttotask", "true")
	}
	if config.Extensions != "" {
		args = append(args, "--extensions", config.Extensions)
	}
	if config.RepoDirName != "" {
		args = append(args, "--repo-name", config.RepoDirName)
	}
	if string(task.PreCommand) != "" {
		args = append(args, "--pre-command", string(task.PreCommand))
	}
	if string(task.PostCommand) != "" {
		args = append(args, "--post-command", string(task.PostCommand))
	}

	args = append(args, "--task-id", task.ID)
	args = append(args, "--task-timeout", fmt.Sprintf("%d", task.Timeout))

	quotedArgs := make([]string, len(args))
	for i, arg := range args {
		quotedArgs[i] = utils.QuotePath(arg)
	}

	cmdStr := utils.QuotePath(exePath) + " " + strings.Join(quotedArgs, " ")
	return buildRepoCommandEnvPrefix() + cmdStr, filepath.Dir(exePath)
}

func (es *ExecutorService) SyncRepoTasks(upsertedIDs []string, deletedIDs []string) {
	for _, id := range upsertedIDs {
		t := es.taskService.GetTaskByID(id)
		if t != nil && utils.DerefBool(t.Enabled, true) && t.TriggerType == constant.TriggerTypeCron {
			_ = es.AddCronTask(t)
		}
	}
	for _, id := range deletedIDs {
		es.RemoveCronTask(id)
	}
}

func (es *ExecutorService) loadEnvVars(taskID string, envIDs string) ([]string, []string) {
	var envs, secrets []string
	if taskID != "" && es.taskService != nil {
		task := es.taskService.GetTaskByID(taskID)
		if task != nil && task.Config != "" {
			var config models.TaskConfig
			if err := json.Unmarshal([]byte(task.Config), &config); err == nil {
				if config.AllEnvs {
					if es.envService != nil {
						envs, secrets = es.envService.GetAllEnvVarsAndSecrets()
					}
				}
			}
		}
	}

	if envs == nil && envIDs != "" && es.envService != nil {
		envs, secrets = es.envService.GetEnvVarsAndSecretsByIDs(envIDs)
	}

	// 自动注入系统 OpenAPI Token（如果配置且启用）
	if es.settingsService != nil {
		tokenJson := es.settingsService.Get(constant.SectionSite, constant.KeyOpenapiToken)
		if tokenJson != "" {
			var tokenConfig struct {
				Token   string `json:"token"`
				Enabled bool   `json:"enabled"`
			}
			if err := json.Unmarshal([]byte(tokenJson), &tokenConfig); err == nil && tokenConfig.Enabled && tokenConfig.Token != "" {
				hasOpenapiToken := false
				for _, e := range envs {
					if strings.HasPrefix(e, "BHPKG_OPENAPI_TOKEN=") || strings.HasPrefix(e, "OPENAPI_TOKEN=") {
						hasOpenapiToken = true
						break
					}
				}
				if !hasOpenapiToken {
					envs = append(envs, "BHPKG_OPENAPI_TOKEN="+tokenConfig.Token)
					secrets = append(secrets, tokenConfig.Token)
				}
			}
		}

		notifyToken := es.settingsService.Get(constant.SectionNotify, constant.KeyNotifyToken)
		if notifyToken != "" {
			hasNotifyToken := false
			for _, e := range envs {
				if strings.HasPrefix(e, "BHPKG_NOTIFY_TOKEN=") {
					hasNotifyToken = true
					break
				}
			}
			if !hasNotifyToken {
				envs = append(envs, "BHPKG_NOTIFY_TOKEN="+notifyToken)
				secrets = append(secrets, notifyToken)
			}
		}
	}

	return envs, secrets
}

func (es *ExecutorService) refreshExecutionRequestEnvs(req *executor.ExecutionRequest, task *models.Task) {
	if task == nil || (req.Type != executor.TaskTypeCron && req.Type != executor.TaskTypeManual) {
		return
	}

	currentEnvs := req.Envs
	envs, secrets := es.loadEnvVars(task.ID, string(task.Envs))
	req.Envs = envs
	req.Secrets = secrets

	for _, ce := range currentEnvs {
		idx := strings.Index(ce, "=")
		if idx == -1 {
			continue
		}
		name := ce[:idx]
		found := false
		for _, ne := range envs {
			if strings.HasPrefix(ne, name+"=") {
				found = true
				break
			}
		}
		if !found {
			req.Envs = append(req.Envs, ce)
		}
	}
}

func (es *ExecutorService) ResolvePath(path string) string {
	absScriptsDir := resolveAbsScriptsDir()
	return strings.ReplaceAll(path, constant.ScriptsDirPlaceholder, absScriptsDir)
}

func buildRepoCommandEnvPrefix() string {
	return utils.BuildShellEnvPrefix(utils.BuildRuntimeProcessEnv())
}

func resolveAbsScriptsDir() string {
	return utils.ResolveAbsScriptsDir()
}
