package v1

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"HelpStudent/config"
	"HelpStudent/core/auth"
	"HelpStudent/core/logx"
	"HelpStudent/core/middleware/response"
	"HelpStudent/internal/app/fastgpt/dao"
	"HelpStudent/internal/app/fastgpt/dto"
	"HelpStudent/internal/app/fastgpt/service"

	"github.com/flamego/binding"
	"github.com/flamego/flamego"
	"github.com/guonaihong/gout"
)

// getFastGPTClient 获取 FastGPT 客户端（使用指定的 API Key）
func getFastGPTClient(apiKey string) *service.FastGPTClient {
	cfg := config.GetConfig()
	return service.NewFastGPTClient(cfg.FastGPT.BaseURL, apiKey)
}

// HandleGetImage 代理图片请求到 FastGPT
// 路由: GET /api/system/img/:imageId
func HandleGetImage(c flamego.Context, r flamego.Render) {
	imageId := c.Param("imageId")
	if imageId == "" {
		response.HTTPFail(r, 400001, "缺少图片ID")
		return
	}

	cfg := config.GetConfig()
	imageURL := cfg.FastGPT.BaseURL + "/system/img/" + imageId

	// 创建 HTTP 客户端请求
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		response.ServiceErr(r, err)
		return
	}

	// 设置请求头，保持与原始请求一致
	req.Header.Set("Accept", "image/*,*/*")
	req.Header.Set("User-Agent", c.Request().Header.Get("User-Agent"))

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		response.ServiceErr(r, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		response.HTTPFail(r, 500001, "FastGPT API 调用失败")
		return
	}

	// 设置响应头
	contentType := resp.Header.Get("Content-Type")
	if contentType != "" {
		c.ResponseWriter().Header().Set("Content-Type", contentType)
	}
	contentLength := resp.Header.Get("Content-Length")
	if contentLength != "" {
		c.ResponseWriter().Header().Set("Content-Length", contentLength)
	}
	// 设置缓存头
	c.ResponseWriter().Header().Set("Cache-Control", "public, max-age=86400")

	// 将图片内容写入响应
	c.ResponseWriter().WriteHeader(http.StatusOK)
	io.Copy(c.ResponseWriter(), resp.Body)
}

// HandleChatCompletion 处理聊天补全请求
func HandleChatCompletion(c flamego.Context, r flamego.Render, req dto.ChatCompletionRequest, errs binding.Errors, authInfo auth.Info) {
	if errs != nil {
		response.InValidParam(r, errs)
		return
	}
	// TODO检查这个用户是否可以使用这个app

	// 根据 fastgptAppId 获取对应的 API Key
	app, err := dao.FastgptApp.GetAppByID(req.FastgptAppId)
	if err != nil {
		logx.SystemLogger.CtxError(c.Request().Context(), err)
		response.HTTPFail(r, 400013, "应用不存在或已禁用")
		return
	}

	// 如果是流式请求，返回提示使用流式接口
	if req.Stream {
		response.HTTPFail(r, 400014, "请使用流式接口 /v1/chat/completions/stream")
		return
	}

	// 非流式请求
	respBody, statusCode, err := getFastGPTClient(app.APIKey).ForwardRequest("POST", "/v1/chat/completions", req)
	if err != nil {
		logx.SystemLogger.CtxError(c.Request().Context(), err)
		response.ServiceErr(r, err)
		return
	}

	if statusCode != http.StatusOK {
		logx.SystemLogger.CtxError(c.Request().Context(), "FastGPT API error: status=%d, body=%s", statusCode, string(respBody))
		response.HTTPFail(r, 500001, "FastGPT API 调用失败")
		return
	}

	// 直接返回 FastGPT 的响应
	c.ResponseWriter().Header().Set("Content-Type", "application/json")
	c.ResponseWriter().WriteHeader(http.StatusOK)
	c.ResponseWriter().Write(respBody)
}

// sendSSEMessage 安全地发送 SSE 消息，捕获可能的 panic
func sendSSEMessage(msg chan<- *dto.SSEMessage, message *dto.SSEMessage) (sent bool) {
	defer func() {
		if r := recover(); r != nil {
			sent = false
		}
	}()
	msg <- message
	return true
}

// HandleStreamChatCompletion 处理流式聊天补全请求（使用 flamego/sse）
func HandleStreamChatCompletion(c flamego.Context, req dto.ChatCompletionRequest, errs binding.Errors, authInfo auth.Info, msg chan<- *dto.SSEMessage) {
	fmt.Println("========== 开始发送消息 ==========")

	if errs != nil {
		sendSSEMessage(msg, &dto.SSEMessage{Data: `{"error":"参数错误"}`, Event: "error"})
		return
	}

	// 根据 fastgptAppId 获取对应的 API Key
	app, err := dao.FastgptApp.GetAppByID(req.FastgptAppId)
	if err != nil {
		logx.SystemLogger.CtxError(c.Request().Context(), err)
		sendSSEMessage(msg, &dto.SSEMessage{Data: `{"error":"应用不存在或已禁用"}`, Event: "error"})
		return
	}

	// 强制设置为流式模式
	req.Stream = true

	// 发起流式请求
	resp, err := getFastGPTClient(app.APIKey).ForwardStreamRequest("POST", "/v1/chat/completions", req)
	if err != nil {
		logx.SystemLogger.CtxError(c.Request().Context(), err)
		sendSSEMessage(msg, &dto.SSEMessage{Data: `{"error":"请求失败"}`, Event: "error"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logx.SystemLogger.CtxError(c.Request().Context(), "FastGPT API error: status=%d", resp.StatusCode)
		sendSSEMessage(msg, &dto.SSEMessage{Data: `{"error":"FastGPT API 调用失败"}`, Event: "error"})
		return
	}

	// 读取并转发流式响应
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		// 跳过空行
		if len(strings.TrimSpace(line)) == 0 {
			continue
		}

		// 解析 SSE 格式数据
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			fmt.Printf("[SSE发送] %s\n", data)
			if !sendSSEMessage(msg, &dto.SSEMessage{Data: data}) {
				// 客户端已断开连接
				fmt.Println("[SSE发送] 客户端已断开连接")
				return
			}

			// 检查是否是结束标记
			if data == "[DONE]" {
				fmt.Println("========== 消息发送完成 ==========")
				break
			}
		}
	}

	if err := scanner.Err(); err != nil {
		logx.SystemLogger.CtxError(c.Request().Context(), "Stream read error", err)
	}
}

// HandleGetHistories 获取聊天历史列表
func HandleGetHistories(c flamego.Context, r flamego.Render, req dto.GetHistoriesRequest, errs binding.Errors, authInfo auth.Info) {
	if errs != nil {
		response.InValidParam(r, errs)
		return
	}

	app, err := dao.FastgptApp.GetAppByID(req.FastgptAppId)
	if err != nil {
		response.ServiceErr(r, err)
		return
	}

	respBody, statusCode, err := getFastGPTClient(app.APIKey).ForwardRequest("POST", "/core/chat/getHistories", req)
	if err != nil {
		response.ServiceErr(r, err)
		return
	}

	if statusCode != http.StatusOK {
		response.HTTPFail(r, 500001, "FastGPT API 调用失败")
		return
	}

	c.ResponseWriter().Header().Set("Content-Type", "application/json")
	c.ResponseWriter().WriteHeader(http.StatusOK)
	c.ResponseWriter().Write(respBody)
}

// HandleUpdateHistory 更新聊天会话
func HandleUpdateHistory(c flamego.Context, r flamego.Render, req dto.UpdateHistoryRequest, errs binding.Errors, authInfo auth.Info) {
	if errs != nil {
		response.InValidParam(r, errs)
		return
	}

	// 使用 appId 获取 API Key
	app, err := dao.FastgptApp.GetAppByID(req.AppId)
	if err != nil {
		logx.SystemLogger.CtxError(c.Request().Context(), err)
		response.ServiceErr(r, err)
		return
	}

	respBody, statusCode, err := getFastGPTClient(app.APIKey).ForwardRequest("POST", "/core/chat/history/updateHistory", req)
	if err != nil {
		logx.SystemLogger.CtxError(c.Request().Context(), err)
		response.ServiceErr(r, err)
		return
	}

	if statusCode != http.StatusOK {
		logx.SystemLogger.CtxError(c.Request().Context(), "FastGPT API error: status=%d, body=%s", statusCode, string(respBody))
		response.HTTPFail(r, 500001, "FastGPT API 调用失败")
		return
	}

	c.ResponseWriter().Header().Set("Content-Type", "application/json")
	c.ResponseWriter().WriteHeader(http.StatusOK)
	c.ResponseWriter().Write(respBody)
}

// HandleGetPaginationRecords 获取聊天记录
func HandleGetPaginationRecords(c flamego.Context, r flamego.Render, req dto.GetPaginationRecordsRequest, errs binding.Errors, authInfo auth.Info) {
	if errs != nil {
		response.InValidParam(r, errs)
		return
	}

	// 使用 fastgptAppId 获取 API Key
	app, err := dao.FastgptApp.GetAppByID(req.FastgptAppId)
	if err != nil {
		logx.SystemLogger.CtxError(c.Request().Context(), err)
		response.ServiceErr(r, err)
		return
	}

	respBody, statusCode, err := getFastGPTClient(app.APIKey).ForwardRequest("POST", "/core/chat/getPaginationRecords", req)
	if err != nil {
		logx.SystemLogger.CtxError(c.Request().Context(), err)
		response.ServiceErr(r, err)
		return
	}

	if statusCode != http.StatusOK {
		logx.SystemLogger.CtxError(c.Request().Context(), "FastGPT API error: status=%d, body=%s", statusCode, string(respBody))
		response.HTTPFail(r, 500001, "FastGPT API 调用失败")
		return
	}

	c.ResponseWriter().Header().Set("Content-Type", "application/json")
	c.ResponseWriter().WriteHeader(http.StatusOK)
	c.ResponseWriter().Write(respBody)
}

// HandleCreateDataset 创建数据集
func HandleCreateDataset(c flamego.Context, r flamego.Render, req dto.DatasetCreateRequest, errs binding.Errors, authInfo auth.Info) {
	if errs != nil {
		response.InValidParam(r, errs)
		return
	}

	app, err := dao.FastgptApp.GetAppByID(req.FastgptAppId)
	if err != nil {
		logx.SystemLogger.CtxError(c.Request().Context(), err)
		response.ServiceErr(r, err)
		return
	}

	respBody, statusCode, err := getFastGPTClient(app.APIKey).ForwardRequest("POST", "/core/dataset/create", req)
	if err != nil {
		logx.SystemLogger.CtxError(c.Request().Context(), err)
		response.ServiceErr(r, err)
		return
	}

	if statusCode != http.StatusOK {
		logx.SystemLogger.CtxError(c.Request().Context(), "FastGPT API error: status=%d, body=%s", statusCode, string(respBody))
		response.HTTPFail(r, 500001, "FastGPT API 调用失败")
		return
	}

	c.ResponseWriter().Header().Set("Content-Type", "application/json")
	c.ResponseWriter().WriteHeader(http.StatusOK)
	c.ResponseWriter().Write(respBody)
}

// HandleListDatasets 列出数据集
func HandleListDatasets(c flamego.Context, r flamego.Render, req dto.DatasetListRequest, errs binding.Errors, authInfo auth.Info) {
	if errs != nil {
		response.InValidParam(r, errs)
		return
	}

	app, err := dao.FastgptApp.GetAppByID(req.FastgptAppId)
	if err != nil {
		logx.SystemLogger.CtxError(c.Request().Context(), err)
		response.ServiceErr(r, err)
		return
	}

	respBody, statusCode, err := getFastGPTClient(app.APIKey).ForwardRequest("POST", "/core/dataset/list", req)
	if err != nil {
		logx.SystemLogger.CtxError(c.Request().Context(), err)
		response.ServiceErr(r, err)
		return
	}

	if statusCode != http.StatusOK {
		logx.SystemLogger.CtxError(c.Request().Context(), "FastGPT API error: status=%d, body=%s", statusCode, string(respBody))
		response.HTTPFail(r, 500001, "FastGPT API 调用失败")
		return
	}

	c.ResponseWriter().Header().Set("Content-Type", "application/json")
	c.ResponseWriter().WriteHeader(http.StatusOK)
	c.ResponseWriter().Write(respBody)
}

// HandleGetDatasetDetail 获取数据集详情
func HandleGetDatasetDetail(c flamego.Context, r flamego.Render, authInfo auth.Info) {
	id := c.Query("id")
	fastgptAppId := c.Query("fastgptAppId")
	if id == "" || fastgptAppId == "" {
		response.HTTPFail(r, 400001, "缺少必要参数")
		return
	}

	app, err := dao.FastgptApp.GetAppByID(fastgptAppId)
	if err != nil {
		logx.SystemLogger.CtxError(c.Request().Context(), err)
		response.ServiceErr(r, err)
		return
	}

	respBody, statusCode, err := getFastGPTClient(app.APIKey).ForwardRequestWithQuery("GET", "/core/dataset/detail", map[string]string{
		"id": id,
	})
	if err != nil {
		logx.SystemLogger.CtxError(c.Request().Context(), err)
		response.ServiceErr(r, err)
		return
	}

	if statusCode != http.StatusOK {
		logx.SystemLogger.CtxError(c.Request().Context(), "FastGPT API error: status=%d, body=%s", statusCode, string(respBody))
		response.HTTPFail(r, 500001, "FastGPT API 调用失败")
		return
	}

	c.ResponseWriter().Header().Set("Content-Type", "application/json")
	c.ResponseWriter().WriteHeader(http.StatusOK)
	c.ResponseWriter().Write(respBody)
}

// HandleDeleteDataset 删除数据集
func HandleDeleteDataset(c flamego.Context, r flamego.Render, authInfo auth.Info) {
	id := c.Query("id")
	fastgptAppId := c.Query("fastgptAppId")
	if id == "" || fastgptAppId == "" {
		response.HTTPFail(r, 400001, "缺少必要参数")
		return
	}

	app, err := dao.FastgptApp.GetAppByID(fastgptAppId)
	if err != nil {
		logx.SystemLogger.CtxError(c.Request().Context(), err)
		response.ServiceErr(r, err)
		return
	}

	respBody, statusCode, err := getFastGPTClient(app.APIKey).ForwardRequestWithQuery("DELETE", "/core/dataset/delete", map[string]string{
		"id": id,
	})
	if err != nil {
		logx.SystemLogger.CtxError(c.Request().Context(), err)
		response.ServiceErr(r, err)
		return
	}

	if statusCode != http.StatusOK {
		logx.SystemLogger.CtxError(c.Request().Context(), "FastGPT API error: status=%d, body=%s", statusCode, string(respBody))
		response.HTTPFail(r, 500001, "FastGPT API 调用失败")
		return
	}

	c.ResponseWriter().Header().Set("Content-Type", "application/json")
	c.ResponseWriter().WriteHeader(http.StatusOK)
	c.ResponseWriter().Write(respBody)
}

// HandleCreateCollectionText 从文本创建集合
func HandleCreateCollectionText(c flamego.Context, r flamego.Render, req dto.CreateCollectionTextRequest, errs binding.Errors, authInfo auth.Info) {
	if errs != nil {
		response.InValidParam(r, errs)
		return
	}

	app, err := dao.FastgptApp.GetAppByID(req.FastgptAppId)
	if err != nil {
		logx.SystemLogger.CtxError(c.Request().Context(), err)
		response.ServiceErr(r, err)
		return
	}

	respBody, statusCode, err := getFastGPTClient(app.APIKey).ForwardRequest("POST", "/core/dataset/collection/create/text", req)
	if err != nil {
		logx.SystemLogger.CtxError(c.Request().Context(), err)
		response.ServiceErr(r, err)
		return
	}

	if statusCode != http.StatusOK {
		logx.SystemLogger.CtxError(c.Request().Context(), "FastGPT API error: status=%d, body=%s", statusCode, string(respBody))
		response.HTTPFail(r, 500001, "FastGPT API 调用失败")
		return
	}

	c.ResponseWriter().Header().Set("Content-Type", "application/json")
	c.ResponseWriter().WriteHeader(http.StatusOK)
	c.ResponseWriter().Write(respBody)
}

// HandleCreateCollectionLink 从链接创建集合
func HandleCreateCollectionLink(c flamego.Context, r flamego.Render, req dto.CreateCollectionLinkRequest, errs binding.Errors, authInfo auth.Info) {
	if errs != nil {
		response.InValidParam(r, errs)
		return
	}

	app, err := dao.FastgptApp.GetAppByID(req.FastgptAppId)
	if err != nil {
		logx.SystemLogger.CtxError(c.Request().Context(), err)
		response.ServiceErr(r, err)
		return
	}

	respBody, statusCode, err := getFastGPTClient(app.APIKey).ForwardRequest("POST", "/core/dataset/collection/create/link", req)
	if err != nil {
		logx.SystemLogger.CtxError(c.Request().Context(), err)
		response.ServiceErr(r, err)
		return
	}

	if statusCode != http.StatusOK {
		logx.SystemLogger.CtxError(c.Request().Context(), "FastGPT API error: status=%d, body=%s", statusCode, string(respBody))
		response.HTTPFail(r, 500001, "FastGPT API 调用失败")
		return
	}

	c.ResponseWriter().Header().Set("Content-Type", "application/json")
	c.ResponseWriter().WriteHeader(http.StatusOK)
	c.ResponseWriter().Write(respBody)
}

// HandlePushData 推送数据到集合
func HandlePushData(c flamego.Context, r flamego.Render, req dto.PushDataRequest, errs binding.Errors, authInfo auth.Info) {
	if errs != nil {
		response.InValidParam(r, errs)
		return
	}

	app, err := dao.FastgptApp.GetAppByID(req.FastgptAppId)
	if err != nil {
		logx.SystemLogger.CtxError(c.Request().Context(), err)
		response.ServiceErr(r, err)
		return
	}

	respBody, statusCode, err := getFastGPTClient(app.APIKey).ForwardRequest("POST", "/core/dataset/data/pushData", req)
	if err != nil {
		logx.SystemLogger.CtxError(c.Request().Context(), err)
		response.ServiceErr(r, err)
		return
	}

	if statusCode != http.StatusOK {
		logx.SystemLogger.CtxError(c.Request().Context(), "FastGPT API error: status=%d, body=%s", statusCode, string(respBody))
		response.HTTPFail(r, 500001, "FastGPT API 调用失败")
		return
	}

	c.ResponseWriter().Header().Set("Content-Type", "application/json")
	c.ResponseWriter().WriteHeader(http.StatusOK)
	c.ResponseWriter().Write(respBody)
}

// HandleSearchTest 搜索测试
func HandleSearchTest(c flamego.Context, r flamego.Render, req dto.SearchTestRequest, errs binding.Errors, authInfo auth.Info) {
	if errs != nil {
		response.InValidParam(r, errs)
		return
	}

	app, err := dao.FastgptApp.GetAppByID(req.FastgptAppId)
	if err != nil {
		logx.SystemLogger.CtxError(c.Request().Context(), err)
		response.ServiceErr(r, err)
		return
	}

	respBody, statusCode, err := getFastGPTClient(app.APIKey).ForwardRequest("POST", "/core/dataset/searchTest", req)
	if err != nil {
		logx.SystemLogger.CtxError(c.Request().Context(), err)
		response.ServiceErr(r, err)
		return
	}

	if statusCode != http.StatusOK {
		logx.SystemLogger.CtxError(c.Request().Context(), "FastGPT API error: status=%d, body=%s", statusCode, string(respBody))
		response.HTTPFail(r, 500001, "FastGPT API 调用失败")
		return
	}

	c.ResponseWriter().Header().Set("Content-Type", "application/json")
	c.ResponseWriter().WriteHeader(http.StatusOK)
	c.ResponseWriter().Write(respBody)
}

// getHDUHelpUserAvatar 从 HDUHelp API 获取用户头像
func getHDUHelpUserAvatar(token string) string {
	type HDUHelpUserResp struct {
		Error int    `json:"error"`
		Msg   string `json:"msg"`
		Data  struct {
			Avatar string `json:"avatar"`
		} `json:"data"`
	}

	var resp HDUHelpUserResp

	err := gout.GET("https://api.hduhelp.com/user/get").
		SetHeader(gout.H{
			"Authorization": "token " + token,
		}).
		SetTimeout(5 * time.Second).
		BindJSON(&resp).
		Do()

	if err != nil {
		logx.SystemLogger.Errorf("HDUHelp get user avatar error: %v", err)
		return ""
	}

	if resp.Error != 0 {
		logx.SystemLogger.Errorf("HDUHelp get user avatar failed: %s", resp.Msg)
		return ""
	}

	return resp.Data.Avatar
}

// HandleOutLinkInit 外链聊天初始化
// 路由: GET /fastgpt/core/chat/outLink/init?chatId=xxx&shareId=xxx&outLinkUid=xxx
func HandleOutLinkInit(c flamego.Context, r flamego.Render, authInfo auth.Info) {
	chatId := c.Query("chatId")
	shareId := c.Query("shareId")
	outLinkUid := c.Query("outLinkUid")
	hduhelpToken := c.Query("hduhelpToken")

	if shareId == "" {
		response.HTTPFail(r, 400001, "缺少必要参数 shareId")
		return
	}

	// 根据 shareId 获取对应的 API Key
	app, err := dao.FastgptApp.GetAppByShareID(shareId)
	if err != nil {
		logx.SystemLogger.CtxError(c.Request().Context(), err)
		response.HTTPFail(r, 400013, "应用不存在或已禁用")
		return
	}

	// 构建查询参数
	queryParams := map[string]string{
		"shareId": shareId,
	}
	if chatId != "" {
		queryParams["chatId"] = chatId
	}
	if outLinkUid != "" {
		queryParams["outLinkUid"] = outLinkUid
	}

	respBody, statusCode, err := getFastGPTClient(app.APIKey).ForwardRequestWithQuery("GET", "/core/chat/outLink/init", queryParams)
	if err != nil {
		logx.SystemLogger.CtxError(c.Request().Context(), err)
		response.ServiceErr(r, err)
		return
	}

	if statusCode != http.StatusOK {
		logx.SystemLogger.CtxError(c.Request().Context(), "FastGPT API error: status=%d, body=%s", statusCode, string(respBody))
		response.HTTPFail(r, 500001, "FastGPT API 调用失败")
		return
	}

	// 如果提供了 hduhelpToken，获取用户头像并替换 respBody 中的 userAvatar
	if hduhelpToken != "" {
		if avatar := getHDUHelpUserAvatar(hduhelpToken); avatar != "" {
			// 解析 JSON 并替换 data.userAvatar 字段np
			var result map[string]interface{}
			if err := json.Unmarshal(respBody, &result); err == nil {
				if data, ok := result["data"].(map[string]interface{}); ok {
					data["userAvatar"] = avatar
					if modifiedBody, err := json.Marshal(result); err == nil {
						respBody = modifiedBody
					}
				}
			}
		}
	}

	c.ResponseWriter().Header().Set("Content-Type", "application/json")
	c.ResponseWriter().WriteHeader(http.StatusOK)
	c.ResponseWriter().Write(respBody)
}

// HandleOutLinkDelHistory 外链删除聊天历史
// 路由: DELETE /api/core/chat/delHistory?appId=xxx&chatId=xxx&shareId=xxx&outLinkUid=xxx
func HandleOutLinkDelHistory(c flamego.Context, r flamego.Render, authInfo auth.Info) {
	fastgptAppId := c.Query("FastgptAppId")
	chatId := c.Query("chatId")
	shareId := c.Query("shareId")
	outLinkUid := c.Query("outLinkUid")

	if shareId == "" || chatId == "" {
		response.HTTPFail(r, 400001, "缺少必要参数 shareId 或 chatId")
		return
	}

	// 根据 shareId 获取对应的 API Key
	app, err := dao.FastgptApp.GetAppByID(fastgptAppId)
	if err != nil {
		response.HTTPFail(r, 400013, "应用不存在或已禁用")
		return
	}

	// 构建查询参数
	queryParams := map[string]string{
		"chatId":  chatId,
		"shareId": shareId,
		"appId":   app.AppId,
	}
	if outLinkUid != "" {
		queryParams["outLinkUid"] = outLinkUid
	}

	respBody, statusCode, err := getFastGPTClient(app.APIKey).ForwardRequestWithQuery("DELETE", "/core/chat/delHistory", queryParams)
	if err != nil {
		response.ServiceErr(r, err)
		return
	}

	if statusCode != http.StatusOK {
		response.HTTPFail(r, 500001, "FastGPT API 调用失败")
		return
	}

	c.ResponseWriter().Header().Set("Content-Type", "application/json")
	c.ResponseWriter().WriteHeader(http.StatusOK)
	c.ResponseWriter().Write(respBody)
}

// HandleGetCollectionQuote 获取集合引用详情
// 路由: POST /fastgpt/core/chat/quote/getCollectionQuote
func HandleGetCollectionQuote(c flamego.Context, r flamego.Render, req dto.GetCollectionQuoteRequest, errs binding.Errors, authInfo auth.Info) {
	if errs != nil {
		response.InValidParam(r, errs)
		return
	}

	app, err := dao.FastgptApp.GetAppByID(req.FastgptAppId)
	if err != nil {
		logx.SystemLogger.CtxError(c.Request().Context(), err)
		response.ServiceErr(r, err)
		return
	}

	respBody, statusCode, err := getFastGPTClient(app.APIKey).ForwardRequest("POST", "/core/chat/quote/getCollectionQuote", req)
	if err != nil {
		logx.SystemLogger.CtxError(c.Request().Context(), err)
		response.ServiceErr(r, err)
		return
	}

	if statusCode != http.StatusOK {
		logx.SystemLogger.CtxError(c.Request().Context(), "FastGPT API error: status=%d, body=%s", statusCode, string(respBody))
		response.HTTPFail(r, 500001, "FastGPT API 调用失败")
		return
	}

	c.ResponseWriter().Header().Set("Content-Type", "application/json")
	c.ResponseWriter().WriteHeader(http.StatusOK)
	c.ResponseWriter().Write(respBody)
}
