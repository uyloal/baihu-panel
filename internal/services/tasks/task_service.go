package tasks

import (
	"strings"

	"github.com/uyloal/baihu-panel/internal/constant"
	"github.com/uyloal/baihu-panel/internal/database"
	"github.com/uyloal/baihu-panel/internal/models"
	"github.com/uyloal/baihu-panel/internal/services/relation"
	"github.com/uyloal/baihu-panel/internal/utils"
)

// TaskParam 任务创建与更新参数传输对象
type TaskParam struct {
	Name          string
	Remark        string
	Command       string
	PreCommand    string
	PostCommand   string
	Tags          string
	Type          string
	Config        string
	Schedule      string
	Timeout       int
	WorkDir       string
	CleanConfig   string
	Envs          string
	TriggerType   string
	RetryCount    int
	RetryInterval int
	RandomRange   int
	SourceID      string
	PinType       string
	Enabled       bool
}

type TaskService struct {
}

func NewTaskService() *TaskService {
	return &TaskService{}
}

func (ts *TaskService) GetTaskBySourceID(sourceID string) *models.Task {
	var task models.Task
	res := database.DB.Where("source_id = ?", sourceID).Limit(1).Find(&task)
	if res.Error != nil || res.RowsAffected == 0 {
		return nil
	}
	ts.loadTagsAndEnvs([]models.Task{task})
	return &task
}

func (ts *TaskService) CreateTask(p *TaskParam) *models.Task {
	if p.Type == "" {
		p.Type = "task"
	}
	if p.TriggerType == "" {
		p.TriggerType = constant.TriggerTypeCron
	}
	if p.PinType == "" {
		p.PinType = constant.PinTypeNone
	}
	task := &models.Task{
		ID:            utils.GenerateID(),
		Name:          p.Name,
		Remark:        p.Remark,
		Command:       models.BigText(p.Command),
		PreCommand:    models.BigText(p.PreCommand),
		PostCommand:   models.BigText(p.PostCommand),
		PinType:       p.PinType,
		Tags:          p.Tags,
		Type:          p.Type,
		TriggerType:   p.TriggerType,
		Config:        models.BigText(p.Config),
		Schedule:      p.Schedule,
		Timeout:       p.Timeout,
		WorkDir:       p.WorkDir,
		CleanConfig:   p.CleanConfig,
		Envs:          models.BigText(p.Envs),
		Enabled:       utils.BoolPtr(true),
		RetryCount:    p.RetryCount,
		RetryInterval: p.RetryInterval,
		RandomRange:   p.RandomRange,
		SourceID:      p.SourceID,
		CreatedAt:     models.Now(),
		UpdatedAt:     models.Now(),
	}
	if p.TriggerType != constant.TriggerTypeCron {
		task.NextRun = nil
	}
	database.DB.Select("*").Create(task)
	relation.DataRelation.SaveTags(task.ID, constant.RelationTypeTaskTag, p.Tags)
	task.Tags = p.Tags
	relation.DataRelation.SaveRelations(task.ID, constant.RelationTypeTaskEnv, p.Envs)
	task.Envs = models.BigText(p.Envs)

	return task
}

func (ts *TaskService) GetTasks() []models.Task {
	var tasks []models.Task
	database.DB.Find(&tasks)
	ts.loadTagsAndEnvs(tasks)
	return tasks
}

// GetTasksWithPagination 分页获取任务列表
func (ts *TaskService) GetTasksWithPagination(page, pageSize int, name string, tags string, taskType string, sortBy string, order string) ([]models.Task, int64) {
	var tasks []models.Task
	var total int64

	query := database.DB.Model(&models.Task{})
	if name != "" {
		query = query.Where("name LIKE ? OR remark LIKE ?", "%"+name+"%", "%"+name+"%")
	}

	if tags != "" {
		tagList := strings.Split(tags, ",")
		var validTags []string
		for _, tag := range tagList {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				validTags = append(validTags, tag)
			}
		}
		if len(validTags) > 0 {
			var storageIDs []string
			database.DB.Model(&models.DataStorage{}).Where("type = ? AND name IN ?", constant.RelationTypeTaskTag, validTags).Pluck("id", &storageIDs)

			var taskIDs []string
			if len(storageIDs) > 0 {
				database.DB.Model(&models.DataRelation{}).Where("type = ? AND relate_id IN ?", constant.RelationTypeTaskTag, storageIDs).Pluck("data_id", &taskIDs)
			}

			if len(taskIDs) > 0 {
				query = query.Where("id IN ?", taskIDs)
			} else {
				query = query.Where("1 = 0")
			}
		}
	}

	if taskType != "" && taskType != "all" {
		query = query.Where("type = ?", taskType)
	}

	sortColumn := "created_at"
	if sortBy != "" {
		switch sortBy {
		case "name", "next_run", "last_run", "created_at", "enabled":
			sortColumn = sortBy
		}
	}
	sortOrder := "DESC"
	if strings.ToUpper(order) == "ASC" {
		sortOrder = "ASC"
	}

	query.Count(&total)
	query.Order("pin_type DESC, " + sortColumn + " " + sortOrder).Offset((page - 1) * pageSize).Limit(pageSize).Find(&tasks)
	ts.loadTagsAndEnvs(tasks)

	return tasks, total
}

func (ts *TaskService) GetTaskByID(id string) *models.Task {
	var task models.Task
	res := database.DB.Where("id = ?", id).Limit(1).Find(&task)
	if res.Error != nil || res.RowsAffected == 0 {
		return nil
	}
	tasks := []models.Task{task}
	ts.loadTagsAndEnvs(tasks)
	return &tasks[0]
}

func (ts *TaskService) UpdateTask(id string, p *TaskParam) *models.Task {
	var task models.Task
	res := database.DB.Where("id = ?", id).Limit(1).Find(&task)
	if res.Error != nil || res.RowsAffected == 0 {
		return nil
	}
	task.Name = p.Name
	task.Remark = p.Remark
	task.Command = models.BigText(p.Command)
	task.PreCommand = models.BigText(p.PreCommand)
	task.PostCommand = models.BigText(p.PostCommand)
	task.PinType = p.PinType
	task.Schedule = p.Schedule
	task.Timeout = p.Timeout
	task.WorkDir = p.WorkDir
	task.CleanConfig = p.CleanConfig
	task.Enabled = &p.Enabled
	task.Config = models.BigText(p.Config)
	task.RetryCount = p.RetryCount
	task.RetryInterval = p.RetryInterval
	task.RandomRange = p.RandomRange
	if p.Type != "" {
		task.Type = p.Type
	}
	if p.TriggerType != "" {
		task.TriggerType = p.TriggerType
	}
	if p.SourceID != "" {
		task.SourceID = p.SourceID
	}

	database.DB.Model(&task).Select(
		"Name", "Remark", "Command", "Tags", "Schedule", "Timeout", "WorkDir",
		"CleanConfig", "Enabled",
		"RetryCount", "RetryInterval", "RandomRange", "Type",
		"TriggerType", "Config", "SourceID", "PinType",
		"PreCommand", "PostCommand",
	).Updates(&task)

	relation.DataRelation.SaveTags(task.ID, constant.RelationTypeTaskTag, p.Tags)
	task.Tags = p.Tags
	relation.DataRelation.SaveRelations(task.ID, constant.RelationTypeTaskEnv, p.Envs)
	task.Envs = models.BigText(p.Envs)

	return &task
}

func (ts *TaskService) DeleteTask(id string) bool {
	database.DB.Where("type = ? AND data_id = ?", constant.BindingTypeTask, id).Delete(&models.NotifyBinding{})
	relation.DataRelation.CleanRelations(id, constant.RelationTypeTaskTag)
	relation.DataRelation.CleanRelations(id, constant.RelationTypeTaskEnv)

	result := database.DB.Where("id = ?", id).Delete(&models.Task{})
	return result.RowsAffected > 0
}

func (ts *TaskService) BatchDeleteTasks(ids []string) int64 {
	database.DB.Where("type = ? AND data_id IN ?", constant.BindingTypeTask, ids).Delete(&models.NotifyBinding{})
	database.DB.Where("type = ? AND data_id IN ?", constant.RelationTypeTaskTag, ids).Delete(&models.DataRelation{})
	database.DB.Where("type = ? AND data_id IN ?", constant.RelationTypeTaskEnv, ids).Delete(&models.DataRelation{})

	result := database.DB.Where("id IN ?", ids).Delete(&models.Task{})
	return result.RowsAffected
}

// GetAllTags 获取所有任务标签
func (ts *TaskService) GetAllTags() ([]string, error) {
	return relation.DataRelation.GetAllTags(constant.RelationTypeTaskTag)
}

func (ts *TaskService) loadTagsAndEnvs(tasks []models.Task) {
	if len(tasks) == 0 {
		return
	}
	taskIDs := make([]string, len(tasks))
	for i, t := range tasks {
		taskIDs[i] = t.ID
	}

	tagsMap := relation.DataRelation.LoadTags(taskIDs, constant.RelationTypeTaskTag)
	envsMap := relation.DataRelation.LoadRelations(taskIDs, constant.RelationTypeTaskEnv)

	for i, t := range tasks {
		if tags, ok := tagsMap[t.ID]; ok {
			tasks[i].Tags = strings.Join(tags, ",")
		} else {
			tasks[i].Tags = ""
		}

		if envs, ok := envsMap[t.ID]; ok {
			tasks[i].Envs = models.BigText(strings.Join(envs, ","))
		} else {
			tasks[i].Envs = models.BigText("")
		}
	}
}
