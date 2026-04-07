---
title: Demos
weight: 2
---

Terminal recordings showing forge in action with LM Studio.

## File Read Task

Model reads `main.go` using the read tool and describes the package.

{{< asciinema key="file-read" cols="100" rows="24" >}}

## File Edit Workflow

Model reads a file, uses the edit tool to replace a word, then reads again to confirm the change.

{{< asciinema key="file-edit" cols="100" rows="24" >}}

Both demos run against [qwen3-coder-next](https://huggingface.co/Qwen) via LM Studio on a local machine.
