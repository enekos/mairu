sed -i '' 's/mapCmd.Run(mapCmd/outputFormat = "json"\n\tmapCmd.Run(mapCmd/g' mairu/internal/cmd/tools_cmd_test.go
sed -i '' 's/sysCmd.Run(sysCmd/outputFormat = "json"\n\tsysCmd.Run(sysCmd/g' mairu/internal/cmd/tools_cmd_test.go
sed -i '' 's/infoCmd.Run(infoCmd/outputFormat = "json"\n\tinfoCmd.Run(infoCmd/g' mairu/internal/cmd/tools_cmd_test.go
