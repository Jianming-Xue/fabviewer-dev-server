// Command server 是 security-management 的组装根：wire 编译期装配全部
// 组件（配置 → 日志 → server 段 → 存储 → 服务 → 引擎/路由 → http.Server），
// main 只做 InitializeServer → Run。存储实现按环境选择：SM_DB_USER 等
// 数据库环境变量齐全 → storage/oracle（生产）；缺省 → storage/memory
// （开发自举，重启即失）。
//
// 启动：
//
//	go run ./cmd/server
//
// 试一下：
//
//	curl -i -X POST -d '{"username":"zhangsan","name":"张三"}' \
//	     -H 'Content-Type: application/json' http://localhost:8081/api/users
//	curl -i http://localhost:8081/api/users?pageIndex=1&pageSize=5
//	curl -i -H 'userName: zhangsan' http://localhost:8081/api/users/{id}
//	curl -i -X DELETE -H 'Accept-Language: zh-CN' http://localhost:8081/api/users/{id}  # 确认消息带姓名
//	curl -i -X DELETE 'http://localhost:8081/api/users/{id}?confirm=true'               # 真正删除
package main

import (
	"fmt"
	"os"

	server "github.com/fab-viewer/common-lib/cmd/server"
	"github.com/fab-viewer/common-lib/common/logx"
)

func main() {
	srv, cleanup, err := InitializeServer()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	code := 0
	if err := server.Run(srv); err != nil {
		code = 1 // Run 已记录 server exited 日志
	}
	logx.Info("bye")
	cleanup() // wire cleanup 链逆序执行：HTTP 优雅停机 → 关存储连接 → 关日志
	os.Exit(code)
}
