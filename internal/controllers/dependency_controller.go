package controllers

import (
	"fmt"
	"os"
	"strings"

	"github.com/uyloal/baihu-panel/internal/models"
	"github.com/uyloal/baihu-panel/internal/models/vo"
	"github.com/uyloal/baihu-panel/internal/services"
	"github.com/uyloal/baihu-panel/internal/services/deps"
	"github.com/uyloal/baihu-panel/internal/utils"

	"github.com/gin-gonic/gin"
)

type DependencyController struct {
	service *services.DependencyService
}

func NewDependencyController() *DependencyController {
	return &DependencyController{
		service: services.NewDependencyService(),
	}
}

// List 获取依赖列表
func (c *DependencyController) List(ctx *gin.Context) {
	depsList, err := c.service.List()
	if err != nil {
		utils.ServerError(ctx, "获取依赖列表失败")
		return
	}
	vos := vo.ToDependencyVOListFromModels(depsList)
	utils.Success(ctx, vos)
}

// Create 添加依赖
func (c *DependencyController) Create(ctx *gin.Context) {
	var req struct {
		Name    string `json:"name" binding:"required"`
		Version string `json:"version"`
		Remark  string `json:"remark"`
	}

	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(ctx, "参数错误")
		return
	}

	dep := &models.Dependency{
		Name:     req.Name,
		Version:  req.Version,
		Language: "node",
		Remark:   req.Remark,
	}

	if err := c.service.Create(dep); err != nil {
		utils.BadRequest(ctx, err.Error())
		return
	}

	utils.Success(ctx, vo.ToDependencyVO(dep))
}

// Delete 删除依赖记录
func (c *DependencyController) Delete(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		utils.BadRequest(ctx, "无效的 ID")
		return
	}

	if err := c.service.Delete(id); err != nil {
		utils.ServerError(ctx, "删除失败")
		return
	}

	utils.SuccessMsg(ctx, "删除成功")
}

// Install 安装依赖 (使用 pnpm add)
func (c *DependencyController) Install(ctx *gin.Context) {
	var req struct {
		Name    string `json:"name" binding:"required"`
		Version string `json:"version"`
		Remark  string `json:"remark"`
	}

	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(ctx, "参数错误")
		return
	}

	dep := &models.Dependency{
		Name:     req.Name,
		Version:  req.Version,
		Language: "node",
		Remark:   req.Remark,
	}

	err := c.service.Install(dep)
	c.service.Create(dep)

	if err != nil {
		utils.ServerError(ctx, err.Error())
		return
	}
	utils.SuccessMsg(ctx, "安装成功")
}

// GetInstallCommand 获取安装命令
func (c *DependencyController) GetInstallCommand(ctx *gin.Context) {
	var req struct {
		Name    string `json:"name" binding:"required"`
		Version string `json:"version"`
	}

	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(ctx, "参数错误")
		return
	}

	dep := &models.Dependency{
		Name:     req.Name,
		Version:  req.Version,
		Language: "node",
	}

	cmd, err := c.service.GetInstallCommand(dep)
	if err != nil {
		utils.ServerError(ctx, err.Error())
		return
	}

	utils.Success(ctx, gin.H{"command": cmd})
}

// GetReinstallAllCommand 获取全部重装命令
func (c *DependencyController) GetReinstallAllCommand(ctx *gin.Context) {
	cmd, err := c.service.GetReinstallAllCommand()
	if err != nil {
		utils.ServerError(ctx, err.Error())
		return
	}

	utils.Success(ctx, gin.H{"command": cmd})
}

// Uninstall 卸载依赖 (使用 pnpm remove)
func (c *DependencyController) Uninstall(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		utils.BadRequest(ctx, "无效的 ID")
		return
	}

	force := ctx.Query("force") == "true"

	depsList, _ := c.service.List()
	var dep *models.Dependency
	for i := range depsList {
		if depsList[i].ID == id {
			dep = &depsList[i]
			break
		}
	}

	if dep == nil {
		utils.NotFound(ctx, "依赖不存在")
		return
	}

	if err := c.service.Uninstall(dep); err != nil {
		if !force {
			utils.ServerError(ctx, err.Error())
			return
		}
	}

	c.service.Delete(id)
	utils.SuccessMsg(ctx, "卸载成功")
}

// Reinstall 重新安装单个依赖
func (c *DependencyController) Reinstall(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		utils.BadRequest(ctx, "无效的 ID")
		return
	}

	depsList, _ := c.service.List()
	var dep *models.Dependency
	for i := range depsList {
		if depsList[i].ID == id {
			dep = &depsList[i]
			break
		}
	}

	if dep == nil {
		utils.NotFound(ctx, "依赖不存在")
		return
	}

	err := c.service.Install(dep)
	c.service.Create(dep)

	if err != nil {
		utils.ServerError(ctx, err.Error())
		return
	}
	utils.SuccessMsg(ctx, "重新安装成功")
}

// ReinstallAll 重新安装所有依赖
func (c *DependencyController) ReinstallAll(ctx *gin.Context) {
	depsList, err := c.service.List()
	if err != nil {
		utils.ServerError(ctx, "获取依赖列表失败")
		return
	}

	var failed []string
	for i := range depsList {
		d := &depsList[i]
		err := c.service.Install(d)
		if err != nil {
			failed = append(failed, d.Name)
		}
		c.service.Create(d)
	}

	if len(failed) > 0 {
		utils.ServerError(ctx, "部分包安装失败: "+strings.Join(failed, ", "))
		return
	}

	utils.SuccessMsg(ctx, "全部重新安装成功")
}

// GetInstalled 获取已安装的包列表 (读取 data/package.json)
func (c *DependencyController) GetInstalled(ctx *gin.Context) {
	packages, err := c.service.GetInstalledPackages()
	if err != nil {
		utils.ServerError(ctx, "获取已安装包失败: "+err.Error())
		return
	}

	utils.Success(ctx, packages)
}

// GetBatchInstallCommand 获取批量安装依赖包的命令
func (c *DependencyController) GetBatchInstallCommand(ctx *gin.Context) {
	var req struct {
		Items []struct {
			Name    string `json:"name" binding:"required"`
			Version string `json:"version"`
		} `json:"items" binding:"required,gt=0"`
	}

	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(ctx, "参数错误: items 不能为空且必须包含 name")
		return
	}

	var depsList []models.Dependency
	for _, item := range req.Items {
		depsList = append(depsList, models.Dependency{
			Name:     item.Name,
			Version:  item.Version,
			Language: "node",
		})
	}

	cmd, err := c.service.GetBatchInstallCommand(depsList)
	if err != nil {
		utils.ServerError(ctx, err.Error())
		return
	}

	utils.Success(ctx, gin.H{"command": cmd})
}

// ParseAndImport 解析上传/粘贴的清单文件内容并批量导入至数据库
func (c *DependencyController) ParseAndImport(ctx *gin.Context) {
	var req struct {
		Content  string `json:"content" binding:"required"`
		ImportDB bool   `json:"import_db"`
	}

	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(ctx, "参数错误: content 必填")
		return
	}

	parsedDeps, err := deps.ParseManifest(req.Content)
	if err != nil {
		utils.ServerError(ctx, "清单文件解析失败: "+err.Error())
		return
	}

	if len(parsedDeps) == 0 {
		utils.BadRequest(ctx, "未解析到任何有效依赖包")
		return
	}

	var finalDeps []models.Dependency
	if req.ImportDB {
		imported, err := c.service.ImportDependencies(parsedDeps)
		if err != nil {
			utils.ServerError(ctx, "导入依赖记录至数据库失败: "+err.Error())
			return
		}
		finalDeps = imported
	} else {
		finalDeps = parsedDeps
	}

	cmd, err := c.service.GetBatchInstallCommand(finalDeps)
	if err != nil {
		utils.ServerError(ctx, "生成安装命令失败: "+err.Error())
		return
	}

	utils.Success(ctx, gin.H{
		"dependencies": vo.ToDependencyVOListFromModels(finalDeps),
		"command":      cmd,
	})
}

// GetDepInstallCommand 获取自动补全的命令，返回给前端执行
func (c *DependencyController) GetDepInstallCommand(ctx *gin.Context) {
	logID := ctx.Query("log_id")
	if logID == "" {
		utils.BadRequest(ctx, "参数错误: log_id 不能为空")
		return
	}

	execPath, err := os.Executable()
	if err != nil {
		execPath = "baihu"
	}

	cmdStr := fmt.Sprintf("%q depinstall %s", execPath, logID)
	utils.Success(ctx, gin.H{
		"command": cmdStr,
	})
}
