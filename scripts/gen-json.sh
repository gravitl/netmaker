#!/bin/bash

go get github.com/mailru/easyjson
go install github.com/mailru/easyjson/...@latest
easyjson -all models/mqtt.go
sed "s/\\*out\\.PresharedKey\\[:\\]/out.PresharedKey[:]/g" -i models/mqtt_easyjson.go
sed "s/\\*in\\.PresharedKey\\[:\\]/in.PresharedKey[:]/g" -i models/mqtt_easyjson.go
