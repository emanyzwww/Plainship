package ui

// Plain 渲染器, 即管道 / 文件 / 测试 buffer 等非终端场景的行为契约.
//
// 实际文本输出由 renderTextLine 承担, 与 Terminal 的唯一差异是 colored() 返回 false:
//
//   - 无色: 全部样式标记被剥离.
//   - 进度静默: Progress / Spinner 不产生任何输出.
//   - 交互放行: Confirm 直接返回 true, Prompt 返回 ErrNonInteractive,
//     调用方应提示改用命令行参数.
//   - 时间戳: 与 Terminal 一致, 由 Options.Timestamp 控制.
//   - JSON: 不属于本渲染器, 见 `renderer_json.go`.
