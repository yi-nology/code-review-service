// Package adapter 提供 ocr.Runner 与 legacy runner 包之间的桥接。
//
// 阶段 A：ocr.Runner 持有 Config 但 exec 仍走 legacy runner.OpenCodeReviewRunner。
// 本文件提供适配器，让 legacy runner 接受 ocr.Config 而非自己的 OpenCodeReviewConfig。
//
// 阶段 A+3 删掉（runner 包整体被吸收）。
// Package adapter 在阶段 A15 完成后清空。
// 原内容（legacyRunner 桥接）已不需要：ocr.Runner 自身实现 legacyRunner 接口。
// 保留本文件为空文件以维持 import 路径稳定。
package adapter