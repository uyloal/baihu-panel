package controllers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/uyloal/baihu-panel/internal/constant"
	"github.com/uyloal/baihu-panel/internal/database"
	"github.com/uyloal/baihu-panel/internal/logger"
	"github.com/uyloal/baihu-panel/internal/models"
	"github.com/uyloal/baihu-panel/internal/models/vo"
	"github.com/uyloal/baihu-panel/internal/services/tasks"
	"github.com/uyloal/baihu-panel/internal/utils"

	"github.com/gin-gonic/gin"
)

type TaskController struct {
	taskService     *tasks.TaskService
	executorService *tasks.ExecutorService
}

func NewTaskController(taskService *tasks.TaskService, executorService *tasks.ExecutorService) *TaskController {
	return &TaskController{
		taskService:     taskService,
		executorService: executorService,
	}
}

// resolveWorkDir 将相对路径转换为绝对路径
func resolveWorkDir(workDir string) string {
	if workDir == "" {
		absPath, err := filepath.Abs(constant.ScriptsWorkDir)
		if err != nil {
			return constant.ScriptsWorkDir
		}
		return absPath
	}
	if strings.HasPrefix(workDir, constant.ScriptsDirPlaceholder) {
		return workDir
	}
	if filepath.IsAbs(workDir) {
		return workDir
	}
	fullPath := filepath.Join(constant.ScriptsWorkDir, workDir)
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return fullPath
	}
	return absPath
}

// isValidDirName 校验目录名是否合法
func isValidDirName(dirName string) bool {
	if dirName == "." || strings.Contains(dirName, "/") || strings.Contains(dirName, "\\") || strings.Contains(dirName, "..") {
		return false
	}
	for _, ch := range dirName {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-' || ch == '.') {
			return false
		}
	}
	return true
}

// getRepoPhysicalPath 计算仓库任务的最终物理绝对路径
func getRepoPhysicalPath(targetPath, dirName, sourceURL, branch string) string {
	if dirName == "." {
		return ""
	}
	finalDirName := dirName
	if finalDirName == "" {
		finalDirName = utils.GetRepoIdentifier(sourceURL, branch)
	}
	if finalDirName == "" {
		return ""
	}

	basePath := targetPath
	if basePath == "" || basePath == constant.ScriptsDirPlaceholder {
		basePath = constant.ScriptsWorkDir
	} else if strings.HasPrefix(basePath, constant.ScriptsDirPlaceholder) {
		basePath = filepath.Join(constant.ScriptsWorkDir, strings.TrimPrefix(basePath, constant.ScriptsDirPlaceholder))
	} else if !filepath.IsAbs(basePath) {
		basePath = filepath.Join(constant.ScriptsWorkDir, basePath)
	}

	fullPath := filepath.Join(basePath, finalDirName)
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return ""
	}
	return absPath
}

// List 获取任务列表
func (tc *TaskController) List(c *gin.Context) {
	p := utils.ParsePagination(c)
	name := c.DefaultQuery("name", "")
	tags := c.DefaultQuery("tags", "")
	taskType := c.DefaultQuery("type", "")

	sortBy := c.DefaultQuery("sort_by", "")
	order := c.DefaultQuery("order", "")

	taskList, total := tc.taskService.GetTasksWithPagination(p.Page, p.PageSize, name, tags, taskType, sortBy, order)
	utils.PaginatedResponse(c, vo.ToTaskVOListFromModels(taskList), total, p)
}

// GetTags 获取所有任务标签
// @Summary 获取所有任务标签
// @Description 获取所有任务中使用的标签列表
// @Tags 任务管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} utils.Response{data=[]string}
// @Router /tasks/tags [get]
func (tc *TaskController) GetTags(c *gin.Context) {
	tags, err := tc.taskService.GetAllTags()
	if err != nil {
		utils.ServerError(c, "获取标签失败")
		return
	}
	utils.Success(c, tags)
}

// Get 获取单个任务
func (tc *TaskController) Get(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		utils.BadRequest(c, "无效的任务ID")
		return
	}

	task := tc.taskService.GetTaskByID(id)
	if task == nil {
		utils.NotFound(c, "任务不存在")
		return
	}

	utils.Success(c, vo.ToTaskVO(task))
}

// Create 创建任务
func (tc *TaskController) Create(c *gin.Context) {
	var req vo.TaskCreateReq

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	if req.Type != constant.TaskTypeRepo && req.Command == "" {
		utils.BadRequest(c, "命令不能为空")
		return
	}

	if req.Schedule != "" {
		if err := tc.executorService.ValidateCron(req.Schedule); err != nil {
			utils.BadRequest(c, "无效的cron表达式: "+err.Error())
			return
		}
	}

	workDir := resolveWorkDir(req.WorkDir)

	var sourceID string
	if req.Type == constant.TaskTypeRepo && req.Config != "" {
		var repoCfg struct {
			SourceURL   string `json:"source_url"`
			Branch      string `json:"branch"`
			RepoDirName string `json:"repo_dir_name"`
			TargetPath  string `json:"target_path"`
		}
		if err := json.Unmarshal([]byte(req.Config), &repoCfg); err == nil && repoCfg.SourceURL != "" {
			if repoCfg.RepoDirName != "" {
				if !isValidDirName(repoCfg.RepoDirName) {
					utils.BadRequest(c, "自定义目录名只能包含字母、数字、下划线、短划线和点，不能只有点，且不能包含路径逻辑")
					return
				}
				sourceID = "repo_" + repoCfg.RepoDirName
			} else {
				sourceID = "repo_" + utils.GetRepoIdentifier(repoCfg.SourceURL, repoCfg.Branch)
			}

			existingTask := tc.taskService.GetTaskBySourceID(sourceID)
			if existingTask != nil {
				utils.BadRequest(c, "当前任务已存在，请检查或更换仓库目录名称")
				return
			}

			newAbsPath := getRepoPhysicalPath(repoCfg.TargetPath, repoCfg.RepoDirName, repoCfg.SourceURL, repoCfg.Branch)
			if newAbsPath != "" {
				if info, err := os.Stat(newAbsPath); err == nil && info.IsDir() {
					utils.BadRequest(c, "本地已存在同名仓库文件夹，请更换自定义目录名或清理残留文件")
					return
				}
			}
		}
	}

	param := tasks.TaskParam{
		Name:          req.Name,
		Remark:        req.Remark,
		Command:       req.Command,
		PreCommand:    req.PreCommand,
		PostCommand:   req.PostCommand,
		Tags:          req.Tags,
		Type:          req.Type,
		Config:        req.Config,
		Schedule:      req.Schedule,
		Timeout:       req.Timeout,
		WorkDir:       workDir,
		CleanConfig:   req.CleanConfig,
		Envs:          req.Envs,
		TriggerType:   req.TriggerType,
		RetryCount:    req.RetryCount,
		RetryInterval: req.RetryInterval,
		RandomRange:   req.RandomRange,
		SourceID:      sourceID,
		PinType:       req.PinType,
		Enabled:       true,
	}

	var task *models.Task
	if sourceID != "" {
		task = tc.taskService.GetTaskBySourceID(sourceID)
		if task != nil {
			task = tc.taskService.UpdateTask(task.ID, &param)
		}
	}

	if task == nil {
		task = tc.taskService.CreateTask(&param)
	}

	tc.executorService.AddCronTask(task)
	utils.Success(c, vo.ToTaskVO(task))
}

// Update 更新任务
func (tc *TaskController) Update(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		utils.BadRequest(c, "无效的任务ID")
		return
	}

	oldTask := tc.taskService.GetTaskByID(id)
	if oldTask == nil {
		utils.NotFound(c, "任务不存在")
		return
	}

	var req vo.TaskUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	if req.Schedule != "" {
		if err := tc.executorService.ValidateCron(req.Schedule); err != nil {
			utils.BadRequest(c, "无效的cron表达式: "+err.Error())
			return
		}
	}

	workDir := resolveWorkDir(req.WorkDir)

	var sourceID string
	if req.Type == constant.TaskTypeRepo && req.Config != "" {
		var repoCfg struct {
			SourceURL   string `json:"source_url"`
			Branch      string `json:"branch"`
			RepoDirName string `json:"repo_dir_name"`
			TargetPath  string `json:"target_path"`
		}
		if err := json.Unmarshal([]byte(req.Config), &repoCfg); err == nil && repoCfg.SourceURL != "" {
			if repoCfg.RepoDirName != "" {
				if !isValidDirName(repoCfg.RepoDirName) {
					utils.BadRequest(c, "自定义目录名只能包含字母、数字、下划线、短划线和点，不能只有点，且不能包含路径逻辑")
					return
				}
				sourceID = "repo_" + repoCfg.RepoDirName
			} else {
				sourceID = "repo_" + utils.GetRepoIdentifier(repoCfg.SourceURL, repoCfg.Branch)
			}

			if sourceID != oldTask.SourceID {
				existingTask := tc.taskService.GetTaskBySourceID(sourceID)
				if existingTask != nil && existingTask.ID != oldTask.ID {
					utils.BadRequest(c, "当前任务已存在，请检查或更换仓库目录名称")
					return
				}
			}

			newAbsPath := getRepoPhysicalPath(repoCfg.TargetPath, repoCfg.RepoDirName, repoCfg.SourceURL, repoCfg.Branch)
			var oldAbsPath string
			if oldTask.Type == constant.TaskTypeRepo && oldTask.Config != "" {
				var oldCfg struct {
					SourceURL   string `json:"source_url"`
					Branch      string `json:"branch"`
					RepoDirName string `json:"repo_dir_name"`
					TargetPath  string `json:"target_path"`
				}
				if json.Unmarshal([]byte(oldTask.Config), &oldCfg) == nil {
					oldAbsPath = getRepoPhysicalPath(oldCfg.TargetPath, oldCfg.RepoDirName, oldCfg.SourceURL, oldCfg.Branch)
				}
			}

			if newAbsPath != "" && newAbsPath != oldAbsPath {
				if info, err := os.Stat(newAbsPath); err == nil && info.IsDir() {
					utils.BadRequest(c, "目标目录在本地已存在同名文件夹，请更换目录名或清理残留文件")
					return
				}
			}
		}
	} else {
		sourceID = oldTask.SourceID
	}

	param := tasks.TaskParam{
		Name:          req.Name,
		Remark:        req.Remark,
		Command:       req.Command,
		PreCommand:    req.PreCommand,
		PostCommand:   req.PostCommand,
		Tags:          req.Tags,
		Type:          req.Type,
		Config:        req.Config,
		Schedule:      req.Schedule,
		Timeout:       req.Timeout,
		WorkDir:       workDir,
		CleanConfig:   req.CleanConfig,
		Envs:          req.Envs,
		TriggerType:   req.TriggerType,
		RetryCount:    req.RetryCount,
		RetryInterval: req.RetryInterval,
		RandomRange:   req.RandomRange,
		SourceID:      sourceID,
		PinType:       req.PinType,
		Enabled:       req.Enabled,
	}

	task := tc.taskService.UpdateTask(id, &param)
	if task == nil {
		utils.NotFound(c, "任务不存在")
		return
	}

	if utils.DerefBool(task.Enabled, true) {
		tc.executorService.AddCronTask(task)
	} else {
		tc.executorService.RemoveCronTask(task.ID)
	}

	utils.Success(c, vo.ToTaskVO(task))
}

// Delete 删除任务
func (tc *TaskController) Delete(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		utils.BadRequest(c, "无效的任务ID")
		return
	}

	task := tc.taskService.GetTaskByID(id)
	if task == nil {
		utils.NotFound(c, "任务不存在")
		return
	}

	deleteFiles := c.Query("delete_files") == "true"
	if deleteFiles && task.Type == constant.TaskTypeRepo {
		tc.deleteRepoPhysicalFiles(task)
	}

	tc.executorService.RemoveCronTask(id)
	tc.executorService.GetScheduler().StopTask(id)

	success := tc.taskService.DeleteTask(id)
	if !success {
		utils.NotFound(c, "任务不存在")
		return
	}

	utils.SuccessMsg(c, "删除成功")
}

// BatchDelete 批量删除任务
func (tc *TaskController) BatchDelete(c *gin.Context) {
	var req struct {
		IDs []string `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	for _, id := range req.IDs {
		tc.executorService.RemoveCronTask(id)
		tc.executorService.GetScheduler().StopTask(id)
	}

	count := tc.taskService.BatchDeleteTasks(req.IDs)
	utils.Success(c, gin.H{"count": count})
}

// BatchDeleteByQuery 按筛选条件批量删除任务
func (tc *TaskController) BatchDeleteByQuery(c *gin.Context) {
	name := c.DefaultQuery("name", "")
	tags := c.DefaultQuery("tags", "")
	taskType := c.DefaultQuery("type", "")

	taskList, total := tc.taskService.GetTasksWithPagination(1, 1000000, name, tags, taskType, "", "")
	if total == 0 {
		utils.Success(c, gin.H{"count": 0})
		return
	}

	ids := make([]string, 0, len(taskList))
	for _, task := range taskList {
		ids = append(ids, task.ID)
		tc.executorService.RemoveCronTask(task.ID)
		tc.executorService.GetScheduler().StopTask(task.ID)
	}

	count := tc.taskService.BatchDeleteTasks(ids)
	utils.Success(c, gin.H{"count": count})
}

// Execute 手动执行任务
func (tc *TaskController) Execute(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		utils.BadRequest(c, "无效的任务ID")
		return
	}

	var req struct {
		Envs []string `json:"envs"`
	}
	_ = c.ShouldBindJSON(&req)

	result := tc.executorService.ExecuteTask(id, req.Envs)
	if !result.Success {
		utils.ServerError(c, result.Error)
		return
	}

	utils.Success(c, vo.ToExecutionResultVO(result))
}

// Stop 停止任务
func (tc *TaskController) Stop(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		utils.BadRequest(c, "无效的ID")
		return
	}

	err := tc.executorService.StopTask(id)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.SuccessMsg(c, "停止请求已发送")
}

// Toggle 切换任务状态
func (tc *TaskController) Toggle(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		utils.BadRequest(c, "无效的任务ID")
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	task := tc.taskService.GetTaskByID(id)
	if task == nil {
		utils.NotFound(c, "任务不存在")
		return
	}

	param := tasks.TaskParam{
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
		Enabled:       req.Enabled,
	}

	updatedTask := tc.taskService.UpdateTask(id, &param)
	if updatedTask == nil {
		utils.NotFound(c, "任务不存在")
		return
	}

	if req.Enabled {
		tc.executorService.AddCronTask(updatedTask)
	} else {
		tc.executorService.RemoveCronTask(updatedTask.ID)
	}

	utils.Success(c, vo.ToTaskVO(updatedTask))
}

// BatchToggle 批量切换任务状态
func (tc *TaskController) BatchToggle(c *gin.Context) {
	var req struct {
		IDs     []string `json:"ids" binding:"required"`
		Enabled bool     `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	for _, id := range req.IDs {
		task := tc.taskService.GetTaskByID(id)
		if task != nil {
			param := tasks.TaskParam{
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
				Enabled:       req.Enabled,
			}
			updatedTask := tc.taskService.UpdateTask(id, &param)
			if updatedTask != nil {
				if req.Enabled {
					tc.executorService.AddCronTask(updatedTask)
				} else {
					tc.executorService.RemoveCronTask(updatedTask.ID)
				}
			}
		}
	}

	utils.SuccessMsg(c, "批量操作成功")
}

// BatchRun 批量运行任务
func (tc *TaskController) BatchRun(c *gin.Context) {
	var req struct {
		IDs []string `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	for _, id := range req.IDs {
		go tc.executorService.ExecuteTask(id, nil)
	}

	utils.SuccessMsg(c, "已触发批量执行")
}

// BatchStop 批量停止任务
func (tc *TaskController) BatchStop(c *gin.Context) {
	var req struct {
		IDs []string `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	for _, id := range req.IDs {
		_ = tc.executorService.StopTask(id)
	}

	utils.SuccessMsg(c, "已触发批量停止")
}

// SyncAllRepos 同步所有仓库任务
func (tc *TaskController) SyncAllRepos(c *gin.Context) {
	tasksList := tc.taskService.GetTasks()
	count := 0
	for _, task := range tasksList {
		if task.Type == constant.TaskTypeRepo && utils.DerefBool(task.Enabled, true) {
			go tc.executorService.ExecuteTask(task.ID, nil)
			count++
		}
	}

	utils.Success(c, gin.H{"count": count})
}

// GetLogs 获取任务历史日志
func (tc *TaskController) GetLogs(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		utils.BadRequest(c, "无效的任务ID")
		return
	}

	p := utils.ParsePagination(c)
	var logs []models.TaskLog
	var total int64

	database.DB.Model(&models.TaskLog{}).Where("task_id = ?", id).Count(&total)
	database.DB.Where("task_id = ?", id).Order("created_at DESC").Offset((p.Page - 1) * p.PageSize).Limit(p.PageSize).Find(&logs)

	utils.PaginatedResponse(c, vo.ToTaskLogVOListFromModels(logs), total, p)
}

// GetLastResult 获取任务最后一次执行结果
func (tc *TaskController) GetLastResult(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		utils.BadRequest(c, "无效的任务ID")
		return
	}

	var taskLog models.TaskLog
	res := database.DB.Where("task_id = ?", id).Order("created_at DESC").Limit(1).Find(&taskLog)
	if res.Error != nil || res.RowsAffected == 0 {
		utils.NotFound(c, "暂无执行记录")
		return
	}

	utils.Success(c, vo.ToTaskLogVO(&taskLog))
}

// ImportTasks 导入任务列表
func (tc *TaskController) ImportTasks(c *gin.Context) {
	var reqs []vo.TaskCreateReq
	if err := c.ShouldBindJSON(&reqs); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	count := 0
	for _, req := range reqs {
		workDir := resolveWorkDir(req.WorkDir)
		param := tasks.TaskParam{
			Name:          req.Name,
			Remark:        req.Remark,
			Command:       req.Command,
			PreCommand:    req.PreCommand,
			PostCommand:   req.PostCommand,
			Tags:          req.Tags,
			Type:          req.Type,
			Config:        req.Config,
			Schedule:      req.Schedule,
			Timeout:       req.Timeout,
			WorkDir:       workDir,
			CleanConfig:   req.CleanConfig,
			Envs:          req.Envs,
			TriggerType:   req.TriggerType,
			RetryCount:    req.RetryCount,
			RetryInterval: req.RetryInterval,
			RandomRange:   req.RandomRange,
			PinType:       req.PinType,
			Enabled:       true,
		}
		task := tc.taskService.CreateTask(&param)
		if task != nil {
			tc.executorService.AddCronTask(task)
			count++
		}
	}

	utils.Success(c, gin.H{"count": count})
}

// BatchUpdateTags 批量更新任务标签
func (tc *TaskController) BatchUpdateTags(c *gin.Context) {
	var req struct {
		IDs  []string `json:"ids" binding:"required"`
		Tags string   `json:"tags"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	for _, id := range req.IDs {
		task := tc.taskService.GetTaskByID(id)
		if task != nil {
			param := tasks.TaskParam{
				Name:          task.Name,
				Remark:        task.Remark,
				Command:       string(task.Command),
				PreCommand:    string(task.PreCommand),
				PostCommand:   string(task.PostCommand),
				Tags:          req.Tags,
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
			}
			tc.taskService.UpdateTask(id, &param)
		}
	}

	utils.SuccessMsg(c, "更新标签成功")
}

// BatchUpdatePin 批量更新置顶
func (tc *TaskController) BatchUpdatePin(c *gin.Context) {
	var req struct {
		IDs     []string `json:"ids" binding:"required"`
		PinType string   `json:"pin_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	for _, id := range req.IDs {
		task := tc.taskService.GetTaskByID(id)
		if task != nil {
			param := tasks.TaskParam{
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
				PinType:       req.PinType,
				Enabled:       utils.DerefBool(task.Enabled, true),
			}
			tc.taskService.UpdateTask(id, &param)
		}
	}

	utils.SuccessMsg(c, "更新置顶成功")
}

// SyncRepoTasks 增量同步仓库任务状态（供本地 reposync 进程调用）
func (tc *TaskController) SyncRepoTasks(c *gin.Context) {
	var req struct {
		RepoID      string   `json:"repo_id"`
		UpsertedIDs []string `json:"upserted_ids"`
		DeletedIDs  []string `json:"deleted_ids"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	tc.executorService.SyncRepoTasks(req.UpsertedIDs, req.DeletedIDs)
	utils.SuccessMsg(c, "增量同步成功")
}

// ToggleTask 切换任务启用/禁用状态
func (tc *TaskController) ToggleTask(c *gin.Context) {
	tc.Toggle(c)
}

// deleteRepoPhysicalFiles 删除仓库关联的物理文件
func (tc *TaskController) deleteRepoPhysicalFiles(task *models.Task) {
	if task.Type != constant.TaskTypeRepo {
		return
	}

	logger.Infof("[Controller] 开始尝试物理删除任务关联文件: %s", task.Name)
	var repoCfg models.RepoConfig
	if err := json.Unmarshal([]byte(task.Config), &repoCfg); err != nil {
		logger.Errorf("[Controller] 解析任务配置失败: %v", err)
		return
	}

	targetPath := repoCfg.TargetPath
	if targetPath == "" {
		repoId := utils.GetRepoIdentifier(repoCfg.SourceURL, repoCfg.Branch)
		if repoId != "" {
			targetPath = repoId
		}
	}

	if targetPath == "" || targetPath == constant.ScriptsDirPlaceholder {
		return
	}

	scriptsDir, _ := filepath.Abs(constant.ScriptsWorkDir)
	fullPath := targetPath
	if strings.HasPrefix(targetPath, constant.ScriptsDirPlaceholder) {
		fullPath = filepath.Join(scriptsDir, strings.TrimPrefix(targetPath, constant.ScriptsDirPlaceholder))
	} else if !filepath.IsAbs(targetPath) {
		fullPath = filepath.Join(scriptsDir, targetPath)
	}

	absTargetPath, _ := filepath.Abs(fullPath)
	rel, err := filepath.Rel(scriptsDir, absTargetPath)
	if err != nil {
		return
	}

	if rel != "." && !strings.HasPrefix(rel, "..") {
		_ = os.RemoveAll(absTargetPath)
		logger.Infof("[Controller] 已成功物理删除文件夹: %s, 路径: %s", task.Name, absTargetPath)
	}
}
