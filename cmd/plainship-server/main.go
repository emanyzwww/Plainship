// Plainship Server 是 Plainship 的服务器端二进制.
// 只包含 serve / token / version 三个命令, 与客户端 (cmd/plainship) 分离.
// 两个二进制共享 internal/version, 版本一一对应.
package main

import "github.com/emanyzwww/plainship/internal/servercli"

func main() {
	servercli.Execute()
}
