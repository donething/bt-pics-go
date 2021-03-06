package worker

import (
	"bt-pics-go/comm"
	"bt-pics-go/conf"
	"bt-pics-go/handlers/tolocal"
	"bt-pics-go/handlers/totg"
	"bt-pics-go/logger"
	"bt-pics-go/yike"
	"os"
	"strings"
	"sync"
)

var (
	// TasksCh 任务通道
	TasksCh chan comm.Album
	WG      sync.WaitGroup
)

// InitWorker 初始化工作协程
func InitWorker(workerCount int) {
	// 创建一个有缓冲的通道来管理工作
	TasksCh = make(chan comm.Album, workerCount)

	// 启动 goroutine 来完成工作
	for id := 1; id <= workerCount; id++ {
		go worker(id)
	}
	logger.Info.Println("[Worker] 工作协程已准备就绪")
}

// 工作
func worker(id int) {
	defer WG.Done()
	// 当程序崩溃时保存进度
	defer func() {
		if err := recover(); err != nil {
			logger.Error.Printf("[Worker%d] 程序崩溃，将保存记录后退出：%s\n", id, err)
			logger.SaveWhenExit()
			os.Exit(0)
		}
	}()

	for {
		// 等待分配工作
		task, ok := <-TasksCh
		if !ok {
			// 这意味着通道已经空了，并且已被关闭
			logger.Info.Printf("[Worker%d] 通道已关闭，完成任务\n", id)
			return
		}

		// 显示我们开始工作了
		// fmt.Printf("Worker: %d : Started %s\n", id, task)

		var err error
		switch conf.Conf.Handler {
		// 发送到一刻相册
		case conf.HandlerToYike:
			err = yike.SendmAlbum(task)
		case conf.HandlerToLocal:
			err = tolocal.Save(task)
		case conf.HandlerToTG:
			err = totg.Send(task)
		default:
			logger.Warn.Printf("[Worker%d] 未知的 Handler：'%s'\n", id, conf.Conf.Handler)
			os.Exit(0)
		}
		if err != nil {
			logger.Error.Printf("%s\n", err)
			logger.LogFail(task)
			continue
		}

		// 仅当成功完成本次下载、发送任务时，才保存进度
		conf.Mu.Lock()
		if *task.IDDonePtr == "" || strings.Compare(task.ID, *task.IDDonePtr) > 0 {
			*task.IDDonePtr = task.ID
			// logger.Info.Printf("记录进度 ID：%s => %s =>%+v\n", task.ID, *task.IDDonePtr, conf.Conf.PicTargets)
		}
		conf.Mu.Unlock()

		// 从失败记录中删除
		if task.IsRetry {
			logger.LogRmFail(task)
		}

		logger.Info.Printf("[Worker%d][%s] 已完成下载、发送专辑'%s'\n", id, task.Tag, task.ID)
	}

	// 显示我们完成了工作
	// fmt.Printf("Worker: %d : Completed %s\n", id, task)
}
