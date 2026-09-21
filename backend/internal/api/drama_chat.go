package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"aishow/internal/chat"
	"aishow/internal/drama"
	"aishow/internal/models"
	"aishow/internal/settings"

	"github.com/gin-gonic/gin"
)

func (s *Server) listChatModels(c *gin.Context) {
	snap := settings.Snapshot(s.db, s.cfg)
	list, err := s.chat.ListModels(snap.ChatURL, settings.ChatToken(s.db, s.cfg))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"url": snap.ChatURL, "models": []string{}, "error": err.Error()})
		return
	}
	list = chat.RankPlotModels(list, snap.ChatModel)
	c.JSON(http.StatusOK, gin.H{"url": snap.ChatURL, "model": chat.PickPlotModel(list, snap.ChatModel), "models": list})
}

func (s *Server) runDramaWrite(c *gin.Context) {
	p, err := s.loadDramaOwned(c, c.Param("id"))
	if err != nil {
		writeErr(c, err)
		return
	}
	if strings.TrimSpace(p.Idea) == "" {
		writeErr(c, badRequest("请先填写题材或意图"))
		return
	}
	unlock := lockDrama(p.ID)
	if err := s.db.First(p, "id = ?", p.ID).Error; err != nil {
		unlock()
		writeErr(c, err)
		return
	}
	if p.Status == models.DramaStatusWriting {
		unlock()
		writeErr(c, badRequest("正在写剧本，请稍候"))
		return
	}
	if err := s.db.Model(p).Updates(map[string]any{
		"status":       models.DramaStatusWriting,
		"write_result": "",
		"updated_at":   time.Now(),
	}).Error; err != nil {
		unlock()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	p.Status = models.DramaStatusWriting
	unlock()
	go s.generateDramaScript(p.ID, p.Idea, p.Style, p.TargetSec)
	s.hydrateDrama(p)
	c.JSON(http.StatusOK, p)
}

func (s *Server) generateDramaScript(projectID, idea, style string, targetSec int) {
	content, model, err := s.dramaChat(false, drama.WriteSystemPrompt(), drama.WriteUserPrompt(idea, style, targetSec), func() string {
		return drama.MockScript(idea, style, targetSec)
	})
	unlock := lockDrama(projectID)
	defer unlock()
	updates := map[string]any{"updated_at": time.Now(), "step": models.DramaStepWrite}
	if err != nil {
		log.Printf("drama write project=%s: %v", projectID, err)
		updates["status"] = models.DramaStatusFailed
		updates["write_result"] = err.Error()
		_ = s.db.Model(&models.DramaProject{}).Where("id = ?", projectID).Updates(updates).Error
		return
	}
	updates["script_text"] = strings.TrimSpace(content)
	updates["write_model"] = model
	updates["write_result"] = ""
	updates["status"] = models.DramaStatusDraft
	_ = s.db.Model(&models.DramaProject{}).Where("id = ?", projectID).Updates(updates).Error
}

func (s *Server) rewriteDramaShot(c *gin.Context) {
	p, err := s.loadDramaOwned(c, c.Param("id"))
	if err != nil {
		writeErr(c, err)
		return
	}
	var body struct {
		Target string `json:"target"`
	}
	_ = c.ShouldBindJSON(&body)
	target := strings.TrimSpace(body.Target)
	if target != "image_prompt" && target != "video_prompt" && target != "continue" {
		writeErr(c, badRequest("请指定重写出图提示、成片提示或衔接优化"))
		return
	}
	idx, err := strconv.Atoi(c.Param("index"))
	if err != nil || idx < 1 {
		writeErr(c, badRequest("无效的镜头序号"))
		return
	}

	unlock := lockDrama(p.ID)
	if err := s.db.First(p, "id = ?", p.ID).Error; err != nil {
		unlock()
		writeErr(c, err)
		return
	}
	shots := drama.ParseShots(p.ShotsJSON)
	if idx > len(shots) {
		unlock()
		writeErr(c, badRequest("无效的镜头序号"))
		return
	}
	shot := shots[idx-1]
	var system, user, source string
	continueFromPrev := idx > 1
	switch target {
	case "continue":
		if idx < 2 {
			unlock()
			writeErr(c, badRequest("第一镜没有上一镜，无法优化衔接"))
			return
		}
		if strings.TrimSpace(shot.VideoPrompt) == "" && strings.TrimSpace(shot.Scene) == "" && strings.TrimSpace(shot.ImagePrompt) == "" {
			unlock()
			writeErr(c, badRequest("请先填写本镜成片提示或画面"))
			return
		}
		system = drama.RewriteContinueSystemPrompt()
		user = drama.RewriteContinueUserPrompt(p, shots[idx-2], shot)
		source = firstNonEmpty(shot.VideoPrompt, shot.Scene, shot.ImagePrompt)
	case "image_prompt":
		if strings.TrimSpace(shot.VideoPrompt) == "" && strings.TrimSpace(shot.Scene) == "" {
			unlock()
			writeErr(c, badRequest("请先填写或修改成片提示/画面"))
			return
		}
		system = drama.RewriteImageSystemPrompt()
		user = drama.RewriteImageUserPrompt(p, shot)
		source = firstNonEmpty(shot.VideoPrompt, shot.Scene, shot.ImagePrompt)
	default:
		if strings.TrimSpace(shot.ImagePrompt) == "" && strings.TrimSpace(shot.Scene) == "" {
			unlock()
			writeErr(c, badRequest("请先填写或修改出图提示/画面"))
			return
		}
		system = drama.RewriteVideoSystemPrompt(continueFromPrev)
		user = drama.RewriteVideoUserPrompt(p, shot, continueFromPrev)
		source = firstNonEmpty(shot.ImagePrompt, shot.Scene, shot.VideoPrompt)
	}
	unlock()

	text, model, err := s.dramaChat(false, system, user, func() string {
		return drama.MockRewrite(target, source, continueFromPrev)
	})
	if err != nil {
		writeErr(c, badRequest(err.Error()))
		return
	}
	text = drama.CleanPromptText(text)
	if text == "" {
		writeErr(c, badRequest("模型没有返回可用提示"))
		return
	}

	unlock = lockDrama(p.ID)
	defer unlock()
	if err := s.db.First(p, "id = ?", p.ID).Error; err != nil {
		writeErr(c, err)
		return
	}
	shots = drama.ParseShots(p.ShotsJSON)
	if idx > len(shots) {
		writeErr(c, badRequest("无效的镜头序号"))
		return
	}
	if target == "image_prompt" {
		shots[idx-1].ImagePrompt = text
	} else {
		if continueFromPrev {
			text = drama.EnsureContinuePrompt(text)
		}
		shots[idx-1].VideoPrompt = text
		if target == "continue" {
			shots[idx-1].ContinueFromPrev = true
		}
	}
	if err := s.saveDramaShots(p, shots, map[string]any{
		"status":           models.DramaStatusDraft,
		"storyboard_model": model,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := s.db.First(p, "id = ?", p.ID).Error; err != nil {
		writeErr(c, err)
		return
	}
	s.hydrateDrama(p)
	c.JSON(http.StatusOK, p)
}

func (s *Server) dramaChat(jsonMode bool, system, user string, mockFn func() string) (content, model string, err error) {
	snap := settings.Snapshot(s.db, s.cfg)
	token := settings.ChatToken(s.db, s.cfg)
	list, _ := s.chat.ListModels(snap.ChatURL, token)
	chatModel := chat.PickPlotModel(list, snap.ChatModel)
	useMock := strings.EqualFold(snap.InferenceMode, "mock")
	if strings.TrimSpace(snap.ChatURL) == "" || chatModel == "" {
		if useMock && mockFn != nil {
			return mockFn(), "mock", nil
		}
		return "", "", fmt.Errorf("请在推理节点填写本地 Chat 地址和模型（Ollama / llama.cpp 等 OpenAI 兼容接口）")
	}
	timeout := 4 * time.Minute
	if jsonMode {
		timeout = 8 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	res, cerr := s.chat.Complete(ctx, snap.ChatURL, token, chatModel, system, user, jsonMode)
	if cerr != nil {
		if useMock && mockFn != nil {
			log.Printf("drama chat fallback mock: %v", cerr)
			return mockFn(), "mock", nil
		}
		return "", "", cerr
	}
	return res.Content, firstNonEmpty(res.Model, chatModel), nil
}
