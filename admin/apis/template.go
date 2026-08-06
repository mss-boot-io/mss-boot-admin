package apis

/*
 * @Author: lwnmengjing
 * @Date: 2022/3/10 22:43
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2022/3/10 22:43
 */

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	git "github.com/go-git/go-git/v5"
	gitHttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/google/uuid"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/controller"

	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/admin/middleware"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
)

func init() {
	e := &Template{}
	response.AppendController(e)
}

type Template struct {
	controller.Simple
	oauthCredentials oauthCredentialStore
	gitClone         func(string, string, string, bool, string) (*git.Repository, error)
	generate         func(*pkg.TemplateConfig) error
	commitAndPush    func(string, string, string, string, *gitHttp.BasicAuth) error
	cleanup          func(...string)
}

func (e Template) clone(url, branch, directory string, noCheckout bool, accessToken string) (*git.Repository, error) {
	if e.gitClone != nil {
		return e.gitClone(url, branch, directory, noCheckout, accessToken)
	}
	return pkg.GitClone(url, branch, directory, noCheckout, accessToken)
}

func (e Template) generateCode(config *pkg.TemplateConfig) error {
	if e.generate != nil {
		return e.generate(config)
	}
	return pkg.Generate(config)
}

func (e Template) push(directory, branch, path, accessToken string, auth *gitHttp.BasicAuth) error {
	if e.commitAndPush != nil {
		return e.commitAndPush(directory, branch, path, accessToken, auth)
	}
	return pkg.CommitAndPushGithubRepo(directory, branch, path, accessToken, auth)
}

func (e Template) scheduleCleanup(paths ...string) {
	if e.cleanup != nil {
		e.cleanup(paths...)
		return
	}
	go func() {
		time.Sleep(10 * time.Minute)
		for _, path := range paths {
			_ = os.RemoveAll(path)
		}
	}()
}

func (Template) Path() string {
	return "template"
}

func (e Template) Other(r *gin.RouterGroup) {
	r.Use(middleware.GetMiddlewares("auth")...)
	r.GET("/template/get-branches", e.GetBranches)
	r.GET("/template/get-path", e.GetPath)
	r.GET("/template/get-params", e.GetParams)
	r.POST("/template/generate", e.Generate)
}

// GetBranches 获取template分支
// @Summary 获取template分支
// @Description 获取template分支
// @Tags generator
// @Accept  application/json
// @Product application/json
// @Param source query string true "template source"
// @Param X-MSS-OAuth-Credential header string false "short-lived OAuth credential handle"
// @Success 200 {object} dto.TemplateGetBranchesResp
// @Router /admin/api/template/get-branches [get]
// @Security Bearer
func (e Template) GetBranches(c *gin.Context) {
	api := response.Make(c)
	req := &dto.TemplateGetBranchesReq{}
	if api.Bind(req).Error != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	if legacyTemplateAccessToken(c, req.AccessToken) {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	accessToken, _, status := e.lookupOAuthIntegrationCredential(c)
	if status != 0 {
		api.Err(status)
		return
	}
	s := strings.Split(req.Source, "/")
	branches, err := pkg.GetGithubRepoAllBranches(c, s[len(s)-2], s[len(s)-1], accessToken)
	if err != nil {
		api.AddError(err).Log.Error("get github branches error")
		api.Err(http.StatusInternalServerError)
		return
	}
	resp := &dto.TemplateGetBranchesResp{
		Branches: make([]string, len(branches)),
	}
	for i := range branches {
		resp.Branches[i] = branches[i].GetName()
	}
	api.OK(resp)
}

// GetPath 获取template文件路径list
// @Summary 获取template文件路径list
// @Description 获取template文件路径list
// @Tags generator
// @Accept  application/json
// @Product application/json
// @Param source query string true "template source"
// @Param branch query string false "branch default:HEAD"
// @Param X-MSS-OAuth-Credential header string false "short-lived OAuth credential handle"
// @Success 200 {object} dto.TemplateGetPathResp
// @Router /admin/api/template/get-path [get]
// @Security Bearer
func (e Template) GetPath(c *gin.Context) {
	api := response.Make(c)
	req := &dto.TemplateGetPathReq{}
	if api.Bind(req).Error != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	if legacyTemplateAccessToken(c, req.AccessToken) {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	accessToken, _, status := e.lookupOAuthIntegrationCredential(c)
	if status != 0 {
		api.Err(status)
		return
	}
	if req.Branch == "" {
		req.Branch = "main"
	}
	//获取模版, 存放位置: templates/provider/owner/repo
	dir := fmt.Sprintf("temp/%s/%s", strings.ReplaceAll(
		strings.ReplaceAll(req.Source, "https://", ""),
		"http://",
		"",
	), req.Branch)
	//获取最新代码
	_, err := e.clone(req.Source, req.Branch, dir, false, accessToken)
	//更新
	if err != nil {
		api.AddError(err).Log.Error("git clone error")
		api.Err(http.StatusInternalServerError)
		return
	}
	defer e.scheduleCleanup(dir)
	resp := &dto.TemplateGetPathResp{}
	resp.Path, err = pkg.GetSubPath(dir)
	for i := range resp.Path {
		if resp.Path[i] == ".git" {
			resp.Path = append(resp.Path[0:i], resp.Path[i+1:]...)
			break
		}
	}
	if err != nil {
		api.AddError(err).Log.Error("get sub path error")
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(resp)
}

// GetParams 获取template参数配置
// @Summary 获取template参数配置
// @Description 获取template参数配置
// @Tags generator
// @Accept  application/json
// @Product application/json
// @Param source query string true "template source"
// @Param branch query string false "branch default:HEAD"
// @Param path query string false "path default:."
// @Param X-MSS-OAuth-Credential header string false "short-lived OAuth credential handle"
// @Success 200 {object} dto.TemplateGetParamsResp
// @Router /admin/api/template/get-params [get]
// @Security Bearer
func (e Template) GetParams(c *gin.Context) {
	api := response.Make(c)
	req := &dto.TemplateGetParamsReq{}
	if api.Bind(req).Error != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	if legacyTemplateAccessToken(c, req.AccessToken) {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	accessToken, _, status := e.lookupOAuthIntegrationCredential(c)
	if status != 0 {
		api.Err(status)
		return
	}
	if req.Branch == "" {
		req.Branch = "main"
	}
	//获取模版, 存放位置: templates/provider/owner/repo
	dir := fmt.Sprintf("temp/%s/%s", strings.ReplaceAll(
		strings.ReplaceAll(req.Source, "https://", ""),
		"http://",
		"",
	), req.Branch)
	//获取最新代码
	_, err := e.clone(req.Source, req.Branch, dir, false, accessToken)
	//更新
	if err != nil {
		api.AddError(err).Log.Error("git clone error")
		api.Err(http.StatusInternalServerError)
		return
	}
	defer e.scheduleCleanup(dir)
	var keys map[string]string
	keys, err = pkg.GetParseFromTemplate(dir, req.Path)
	if err != nil {
		api.AddError(err).Log.Error("get parse from template error")
		api.Err(http.StatusFailedDependency)
		return
	}
	resp := &dto.TemplateGetParamsResp{
		Params: make([]dto.TemplateParam, 0, len(keys)),
	}

	for k, v := range keys {
		resp.Params = append(resp.Params, dto.TemplateParam{
			Name: k,
			Tip:  v,
		})
	}

	api.OK(resp)
}

// Generate 从模版生成代码
// @Summary 从模版生成代码
// @Description 从模版生成代码
// @Tags generator
// @Accept  application/json
// @Product application/json
// @Param data body dto.TemplateGenerateReq true "data"
// @Param X-MSS-OAuth-Credential header string false "short-lived OAuth credential handle"
// @Success 200 {object} dto.TemplateGenerateResp
// @Router /admin/api/template/generate [post]
// @Security Bearer
func (e Template) Generate(c *gin.Context) {
	api := response.Make(c)
	req := &dto.TemplateGenerateReq{}
	if api.Bind(req).Error != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	if legacyTemplateAccessToken(c, req.AccessToken) {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	accessToken, credentialHandle, status := e.lookupOAuthIntegrationCredential(c)
	if status != 0 {
		api.Err(status)
		return
	}

	if req.Template.Branch == "" {
		req.Template.Branch = "main"
	}
	//获取模版, 存放位置: temp/provider/owner/repo
	dir := fmt.Sprintf("temp/%s/%s", strings.ReplaceAll(
		strings.ReplaceAll(req.Template.Source, "https://", ""),
		"http://",
		"",
	), req.Template.Branch)
	//获取新代码
	_, err := e.clone(
		req.Template.Source,
		req.Template.Branch, dir, false,
		accessToken)
	if err != nil {
		api.AddError(err).Log.Error("git clone error")
		api.Err(http.StatusInternalServerError)
		return
	}

	//获取目的提交项目，存放路径: temp/provider/owner/repo/branch
	branch := fmt.Sprintf("generate/%s", uuid.New().String())
	codeDir := fmt.Sprintf("temp/%s/%s", strings.ReplaceAll(
		strings.ReplaceAll(req.Generate.Repo, "https://", ""),
		"http://",
		"",
	), branch)

	_, err = e.clone(req.Generate.Repo, "", codeDir, false, accessToken)
	if err != nil {
		api.AddError(err).Log.Error("git clone error")
		api.Err(http.StatusInternalServerError)
		return
	}
	defer e.scheduleCleanup(dir, codeDir)
	destination := codeDir
	if req.Generate.Service != "" {
		destination = filepath.Join(destination, req.Generate.Service)
	}
	err = e.generateCode(&pkg.TemplateConfig{
		Service:       req.Template.Path,
		TemplateLocal: dir,
		//TemplateLocalSubPath: req.Template.Branch,
		Destination: destination,
		Params:      req.Generate.Params,
	})
	if err != nil {
		api.AddError(err).Log.Error("generate error")
		api.Err(http.StatusInternalServerError)
		return
	}
	err = e.push(codeDir, branch, req.Generate.Service, accessToken,
		&gitHttp.BasicAuth{
			Username: req.Email,
			Password: accessToken,
		})
	if err != nil {
		api.AddError(err).Log.Error("commit and push github repo error")
		api.Err(http.StatusInternalServerError)
		return
	}
	if status = e.deleteOAuthIntegrationCredential(c, credentialHandle); status != 0 {
		api.Err(status)
		return
	}
	resp := &dto.TemplateGenerateResp{
		Repo:   req.Generate.Repo,
		Branch: branch,
	}
	api.OK(resp)
}

//// getGithubConfig 获取github配置
//func getGithubConfig(c *gin.Context) (g *models.Github, err error) {
//	user := middlewares.GetLoginUser(c)
//	if user == nil {
//		return nil, errors.New("user not found")
//	}
//	//todo 需要改成各人的
//	return models.GetMyGithubConfig(c, "lwnmengjing@qq.com")
//}
