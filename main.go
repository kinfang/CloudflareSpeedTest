package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"runtime"
	"time"

	"CloudflareSpeedTest/task"
	"CloudflareSpeedTest/utils"
)

var (
	version, versionNew string
)

func init() {
	var printVersion bool
	var help = `
CloudflareSpeedTest ` + version + `
测试 Cloudflare CDN 所有 IP 的延迟和速度，获取最快 IP (IPv4+IPv6)！
https://github.com/XIU2/CloudflareSpeedTest

参数：
    -n 200
        测速线程数量；越多测速越快，性能弱的设备 (如路由器) 请勿太高；(默认 200 最多 1000)
    -t 4
        延迟测速次数；单个 IP 延迟测速次数，为 1 时将过滤丢包的IP，TCP协议；(默认 4)
    -tp 443
        延迟测速端口；延迟测速 TCP 协议的端口；(默认 443)
    -dn 20
        下载测速数量；延迟测速并排序后，从最低延迟起下载测速的数量；(默认 20)
    -dt 10
        下载测速时间；单个 IP 下载测速最长时间，单位：秒；(默认 10)
    -url https://cf.xiu2.xyz/Github/CloudflareSpeedTest.png
        下载测速地址；用来下载测速的 Cloudflare CDN 文件地址，如地址含有空格请加上引号；
    -tl 200
        平均延迟上限；只输出低于指定平均延迟的 IP，可与其他上限/下限搭配；(默认 9999 ms)
    -tll 40
        平均延迟下限；只输出高于指定平均延迟的 IP，可与其他上限/下限搭配，过滤被假蔷的 IP；(默认 0 ms)
    -sl 5
        下载速度下限；只输出高于指定下载速度的 IP，凑够指定数量 [-dn] 才会停止测速；(默认 0.00 MB/s)
    -p 20
        显示结果数量；测速后直接显示指定数量的结果，为 0 时不显示结果直接退出；(默认 20)
    -f ip.txt
        IP段数据文件；如路径含有空格请加上引号；支持其他 CDN IP段；(默认 ip.txt)
    -o result.csv
        写入结果文件；如路径含有空格请加上引号；值为空时不写入文件 [-o ""]；(默认 result.csv)
    -dd
        禁用下载测速；禁用后测速结果会按延迟排序 (默认按下载速度排序)；(默认 启用)
    -ipv6
        IPv6测速模式；确保 IP 段数据文件内只包含 IPv6 IP段，软件不支持同时测速 IPv4+IPv6；(默认 IPv4)
    -allip
        测速全部的IP；对 IP 段中的每个 IP (仅支持 IPv4) 进行测速；(默认 每个 IP 段随机测速一个 IP)
    -v
        打印程序版本+检查版本更新
    -h
        打印帮助说明
`

	flag.IntVar(&task.Routines, "n", 200, "测速线程数量")
	flag.IntVar(&task.PingTimes, "t", 4, "延迟测速次数")
	flag.IntVar(&task.TCPPort, "tp", 443, "延迟测速端口")
	flag.DurationVar(&utils.InputMaxDelay, "tl", 9999*time.Millisecond, "平均延迟上限")
	flag.DurationVar(&utils.InputMinDelay, "tll", time.Duration(0), "平均延迟下限")
	flag.DurationVar(&task.Timeout, "dt", 10*time.Second, "下载测速时间")
	flag.IntVar(&task.TestCount, "dn", 20, "下载测速数量")
	flag.StringVar(&task.URL, "url", "https://cf.xiu2.xyz/Github/CloudflareSpeedTest.png", "下载测速地址")
	flag.BoolVar(&task.Disable, "dd", false, "禁用下载测速")
	flag.BoolVar(&task.IPv6, "ipv6", false, "启用IPv6")
	flag.BoolVar(&task.TestAll, "allip", false, "测速全部 IP")
	flag.StringVar(&task.IPFile, "f", "ip.txt", "IP 数据文件")
	flag.Float64Var(&task.MinSpeed, "sl", 0, "下载速度下限")
	flag.IntVar(&utils.PrintNum, "p", 20, "显示结果数量")
	flag.StringVar(&utils.Output, "o", "result.csv", "输出结果文件")
	flag.BoolVar(&printVersion, "v", false, "打印程序版本")

	flag.Usage = func() { fmt.Print(help) }
	flag.Parse()
	if printVersion {
		println(version)
		fmt.Println("检查版本更新中...")
		checkUpdate()
		if versionNew != "" {
			fmt.Println("发现新版本 [" + versionNew + "]！请前往 [https://github.com/XIU2/CloudflareSpeedTest] 更新！")
		} else {
			fmt.Println("当前为最新版本 [" + version + "]！")
		}
		os.Exit(0)
	}
}

func main() {
	go checkUpdate()                          // 检查版本更新
	initRandSeed()                            // 置随机数种子
	ips := loadFirstIPOfRangeFromFile(task.IPFile) // 读入IP

	// 开始延迟测速
	fmt.Printf("# XIU2/CloudflareSpeedTest %s \n", version)
	ipVersion := "IPv4"
	if task.IPv6 { // IPv6 模式判断
		ipVersion = "IPv6"
	}
	fmt.Printf("开始延迟测速（模式：TCP %s，端口：%d ，平均延迟上限：%v，平均延迟下限：%v)\n", ipVersion, task.TCPPort, utils.InputMaxDelay, utils.InputMinDelay)

	pingData := task.NewPing(ips).Run().FilterDelay()
	speedData := task.TestDownloadSpeed(pingData)
	utils.ExportCsv(speedData)
	speedData.Print(task.IPv6)

	if versionNew != "" {
		fmt.Printf("\n*** 发现新版本 [%s]！请前往 [https://github.com/XIU2/CloudflareSpeedTest] 更新！ ***\n", versionNew)
	}

	if utils.Output != "" {
		fmt.Printf("完整测速结果已写入 %v 文件，请使用记事本/表格软件查看。\n", utils.Output)
	}
	if runtime.GOOS == "windows" { // 如果是 Windows 系统，则需要按下 回车键 或 Ctrl+C 退出（避免通过双击运行时，测速完毕后直接关闭）
		fmt.Printf("按下 回车键 或 Ctrl+C 退出。\n")
		var pause int
		fmt.Scanln(&pause)
	}

	// control := make(chan bool, pingRoutine)
	// for _, ip := range ips {
	// 	wg.Add(1)
	// 	control <- false
	// 	handleProgress := handleProgressGenerator(bar) // 多线程进度条
	// 	go tcpingGoroutine(&wg, &mu, ip, tcpPort, pingTime, &data, control, handleProgress)
	// }
	// wg.Wait()
	// bar.Finish()
	// data := ping.Data()
	// sort.Sort(utils.CloudflareIPDataSet(data))
	// sort.Sort(CloudflareIPDataSet(data)) // 排序（按延迟，从低到高，不同丢包率会分开单独按延迟和丢包率排序）

	// 延迟测速完毕后，以 [平均延迟上限] + [平均延迟下限] 条件过滤结果
	// if timeLimit != 9999 || timeLimitLow != 0 {
	// 	for i := 0; i < len(data); i++ {
	// 		if float64(data[i].pingTime) > timeLimit { // 平均延迟上限
	// 			break
	// 		}
	// 		if float64(data[i].pingTime) <= timeLimitLow { // 平均延迟下限
	// 			continue
	// 		}
	// 		data2 = append(data2, data[i]) // 延迟满足条件时，添加到新数组中
	// 	}
	// 	data = data2
	// 	data2 = []CloudflareIPData{}
	// }

	// 开始下载测速
	// if !disableDownload { // 如果禁用下载测速就跳过
	// 	testDownloadSpeed(data, data2, bar)
	// }

	// if len(data2) > 0 { // 如果该数组有内容，说明指定了 [下载测速下限] 条件，且最少有 1 个满足条件的 IP
	// 	data = data2
	// }
	// sort.Sort(CloudflareIPDataSetD(data)) // 排序（按下载速度，从高到低）
	// if outputFile != "" {
	// 	ExportCsv(outputFile, data) // 输出结果到文件
	// }
	// printResult(data) // 显示最快结果
}

// 检查更新
func checkUpdate() {
	timeout := time.Duration(10 * time.Second)
	client := http.Client{Timeout: timeout}
	res, err := client.Get("https://api.xiuer.pw/ver/cloudflarespeedtest.txt")
	if err != nil {
		return
	}
	// 读取资源数据 body: []byte
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return
	}
	// 关闭资源流
	defer res.Body.Close()
	if string(body) != version {
		versionNew = string(body)
	}
}
