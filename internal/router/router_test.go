package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/uyloal/baihu-panel/internal/constant"
	"github.com/uyloal/baihu-panel/internal/database"
	"github.com/uyloal/baihu-panel/internal/models"
	"github.com/uyloal/baihu-panel/internal/services"
	"github.com/uyloal/baihu-panel/internal/systime"
	"github.com/uyloal/baihu-panel/internal/utils"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) {
	time.Local = systime.CST
	_, _ = services.LoadConfig("")
	db, err := gorm.Open(sqlite.Open("file:memdb_router?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("无法开启 SQLite 内存测试库: %v", err)
	}

	database.DB = db

	if err := database.Migrate(); err != nil {
		t.Fatalf("测试数据迁移失败: %v", err)
	}
}

func TestRouterSetup(t *testing.T) {
	setupTestDB(t)

	controllers := RegisterControllers()
	if controllers == nil {
		t.Fatal("RegisterControllers 返回 nil")
	}
	defer StopCron()

	engine := Setup(controllers)
	if engine == nil {
		t.Fatal("Setup 返回 nil")
	}

	// 验证路由列表中包含核心路由
	routes := engine.Routes()
	expectedRoutes := map[string]string{
		"/api/v1/dashboard/stats":      "GET",
		"/api/v1/dashboard/sentence":   "GET",
		"/api/v1/dashboard/sendstats":  "GET",
		"/api/v1/dashboard/taskstats":  "GET",
		"/api/v1/tasks/tags":           "GET",
		"/api/v1/tasks/batch-by-query": "DELETE",
		"/api/v1/tasks/:id/execute":    "POST",
		"/api/v1/tasks/:id/stop":       "POST",
		"/api/v1/notify/send":          "POST",
		"/open2api/v1/tasks":           "GET",
	}

	registered := make(map[string]bool)
	for _, r := range routes {
		registered[r.Method+" "+r.Path] = true
	}

	for path, method := range expectedRoutes {
		key := method + " " + path
		if !registered[key] {
			t.Errorf("期望注册路由 %s 但未找到", key)
		}
	}
}

func TestDashboardTaskStats(t *testing.T) {
	setupTestDB(t)

	controllers := RegisterControllers()
	defer StopCron()

	engine := Setup(controllers)

	// 获取 admin 用户生成认证 Token
	var admin models.User
	database.DB.Where("username = ?", "admin").First(&admin)
	token, _ := utils.GenerateToken(admin.ID, admin.Username, admin.TokenVersion, 7, constant.Secret)

	// 插入测试数据
	taskID := "task_test_1"
	now := models.Now()
	database.DB.Create(&models.Task{
		ID:   taskID,
		Name: "每日打卡任务",
	})
	database.DB.Create(&models.TaskLog{
		ID:        "log_test_1",
		TaskID:    taskID,
		Status:    "success",
		Duration:  120,
		StartTime: &now,
		EndTime:   &now,
		CreatedAt: now,
	})

	// 测试 GET /api/v1/dashboard/taskstats?days=30
	req, _ := http.NewRequest("GET", "/api/v1/dashboard/taskstats?days=30", nil)
	req.AddCookie(&http.Cookie{Name: constant.CookieName, Value: token})
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("taskstats 期望状态码 200, 实际: %d, body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Code int `json:"code"`
		Data []struct {
			TaskID   string `json:"task_id"`
			TaskName string `json:"task_name"`
			Count    int    `json:"count"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Data) == 0 {
		t.Fatalf("taskstats 期望返回统计数据，实际为空: %s", w.Body.String())
	}
	if resp.Data[0].TaskID != taskID || resp.Data[0].TaskName != "每日打卡任务" || resp.Data[0].Count < 1 {
		t.Fatalf("taskstats 返回数据不正确: %+v", resp.Data[0])
	}

	// 测试 GET /api/v1/dashboard/sendstats?days=30
	req2, _ := http.NewRequest("GET", "/api/v1/dashboard/sendstats?days=30", nil)
	req2.AddCookie(&http.Cookie{Name: constant.CookieName, Value: token})
	w2 := httptest.NewRecorder()
	engine.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("sendstats 期望状态码 200, 实际: %d, body: %s", w2.Code, w2.Body.String())
	}

	// 测试 GET /api/v1/dashboard/stats
	req3, _ := http.NewRequest("GET", "/api/v1/dashboard/stats", nil)
	req3.AddCookie(&http.Cookie{Name: constant.CookieName, Value: token})
	w3 := httptest.NewRecorder()
	engine.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("stats 期望状态码 200, 实际: %d, body: %s", w3.Code, w3.Body.String())
	}
}
