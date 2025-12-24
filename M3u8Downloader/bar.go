package M3u8Downloader

import (
	"fmt"
	"os"
	"unicode/utf8"
)

// 30  40  黑色
// 31  41  红色
// 32  42  绿色
// 33  43  黄色
// 34  44  蓝色
// 35  45  紫红色
// 36  46  青蓝色
// 37  47  白色
const (
	LinuxTerminal        ModelType = true
	WindowsCmd           ModelType = false
	Highlight            ShowType  = 1
	Flicker              ShowType  = 5
	BasicCompletedIcon   rune      = '█'
	BasicUnCompletedIcon byte      = ' '
)

// 前景颜色
const (
	FrontBlack PrintColor = 30 + iota
	FrontRed
	FrontGreen
	FrontYellow
	FrontBlue
	FrontAmaranth
	FrontUltramarine
	FrontWhite
)

// 背景颜色
const (
	BackBlack PrintColor = 40 + iota
	BackRed
	BackGreen
	BackYellow
	BackBlue
	BackAmaranth
	BackUltramarine
	BackWhite
)

var (
	printModel = map[ModelType]string{
		LinuxTerminal: "\rProgress:[\u001B[%d;%d;%dm%s\u001B[0m] %3.2f%% Completed:[%3d] Total:[%3d]",
		WindowsCmd:    "\rProgress:[%s] %3.2f%% Completed:[%3d] Total:[%3d]",
	}
)

type ShowType int
type PrintColor int
type ModelType bool
type ModelFunc func([]byte, float32, int64, int64)

type Bar interface {
	// Play 执行一次记录
	Update(cur int64)
	// Finish 完成处理方法，在完成后会将相关数据归零，以便重新使用
	Finish()
	// Setting 设置信息方法，通过此方法调用其他设置方法
	Setting() *BarConfig
	// UpdateConfig 更新配置信息对象
	UpdateConfig(newConfig *BarConfig)
	// ReSetRange 重新设置范围
	ReSetRange(start, total int64)
}

// X  0  终端默认设置
//
//	1  高亮显示
//
// X  4  使用下划线
//
//	5  闪烁
//
// X  7  反白显示
// X  8  不可见
type __bar struct {
	percent         float32 //百分比
	currentPosition int64   //当前进度位置
	total           int64   //总进度
	bar             []byte  //进度条
	left            int
	right           int
	config          *BarConfig //进度条配置
}

type BarConfig struct {
	completedIcon   rune       //已完成的图案
	unCompletedIcon byte       //未完成的图案
	frontColor      PrintColor // 前景色
	backColor       PrintColor //背景色
	showWay         ShowType   //显示方式
	printModel      ModelType  //打印模式
	printFunc       ModelFunc  //两种打印模式下的打印函数
}

// NewBar 创建进度条对象，显示模式默认为在Linux Terminal下
// 默认范围:[0，total),默认显示图案：=，默认显示颜色：绿色（32）
func NewBar(total int64) Bar {
	return NewOptionWithGraphAndModel(0, total, BasicCompletedIcon, LinuxTerminal)
}

// NewOptionWithGraphAndModel 创建自定义范围、图案的进度条对象，并自定义显示模式
func NewOptionWithGraphAndModel(start, total int64, completedIcon rune, model ModelType) Bar {
	config := NewDefaultBarConfig()
	config.completedIcon = completedIcon
	config.printModel = model
	return NewBarWithConfig(start, total, config)
}

// NewDefaultBarConfig 创建默认配置对象
func NewDefaultBarConfig() *BarConfig {
	return &BarConfig{
		completedIcon: BasicCompletedIcon,
		//unCompletedIcon: BasicUnCompletedIcon,
		frontColor: FrontGreen,
		backColor:  BackBlack,
		showWay:    Highlight,
		printModel: LinuxTerminal,
		printFunc:  nil,
	}
}

// NewBarWithConfig 通过自定义配置对象创建进度条对象
func NewBarWithConfig(start, total int64, config *BarConfig) Bar {
	b := &__bar{
		currentPosition: start,
		total:           total,
		percent:         0.0,
		bar:             make([]byte, 201),
		config:          config,
		left:            0,
		right:           50,
	}
	b.config.SetShowModel(b.config.printModel)
	//fill the area of uncompleted with the uncompleted icon
	for i := 0; i < 50; i++ {
		b.bar[i] = BasicUnCompletedIcon //b.config.unCompletedIcon
	}
	return b
}

// Setting 设置入口
func (b *__bar) Setting() *BarConfig {
	return b.config
}

// UpdateConfig 更新配置信息，将新的结构体传入，替换原有的结构体
func (b *__bar) UpdateConfig(newConfig *BarConfig) {
	b.config = newConfig
}

// SetShowModel 设置显示模式
func (bc *BarConfig) SetShowModel(model ModelType) {
	bc.printModel = model
	if model {
		bc.printFunc = bc.printInLinux
	} else {
		bc.printFunc = bc.printInWindows
	}
}

// SetCompletedIcon 设置已完成进度的显示图案
func (bc *BarConfig) SetCompletedIcon(icon rune) {
	if utf8.RuneLen(icon) < 0 {
		bc.completedIcon = BasicCompletedIcon
	} else {
		bc.completedIcon = icon
	}
}

//// SetUnCompletedIcon 设置未完成进度的显示图案
//func (bc *BarConfig) SetUnCompletedIcon(icon byte) {
//	bc.unCompletedIcon = icon
//}

// SetCursorIcon 设置游标的显示图案
func (bc *BarConfig) SetCursorIcon(icon rune) {
	bc.completedIcon = icon
}

// SetFrontColorInTerminal 设置显示的前景颜色
// 若颜色值异常，则默认为绿色
func (bc *BarConfig) SetFrontColorInTerminal(color PrintColor) {
	if color < 30 || color > 37 {
		color = 32
	}
	bc.frontColor = color
}

// SetBackColorInTerminal 设置显示的前景颜色
// 若颜色值异常，则默认为绿色
func (bc *BarConfig) SetBackColorInTerminal(color PrintColor) {
	if color < 40 || color > 47 {
		color = 40
	}
	bc.backColor = color
}

// ReSetRange 重新设置范围
func (b *__bar) ReSetRange(start, total int64) {
	b.currentPosition = start
	b.total = total
}

// getPercent 获取当前状态百分比
func (b *__bar) getPercent() float32 {
	return float32(b.currentPosition) / float32(b.total) * 100
}

// Update 执行一次记录
func (b *__bar) Update(cur int64) {
	b.currentPosition = cur
	latest := b.getPercent()
	//如果写成：int(latest-b.percent)会损失很多精度，导致最后进度条无法到达终点
	var temp = int(latest) - int(b.percent)
	if b.percent != latest && temp%2 == 0 {
		b.percent = latest
		temp = temp >> 1 //(temp/2)
		var start, n, i int
		start = b.right
		for i = 0; i < temp; i++ {
			n = utf8.EncodeRune(b.bar[b.left:b.left+utf8.UTFMax], b.config.completedIcon)
			b.left += n
			b.right += n - 1
		}
		b.excursion(start, b.right+n-1)
	}
	//将bar的[0:右边界]打印出来，以如下的方式传参，不会发生内存拷贝，仍然是同一地址
	b.config.printFunc(b.bar[:b.right], b.percent, b.currentPosition, b.total)
}

// excursion 补充偏移的位置
func (b *__bar) excursion(start, end int) {
	for i := start; i < end; i++ {
		if int(b.bar[i]) < 100 {
			b.bar[i] = BasicUnCompletedIcon //b.config.unCompletedIcon
		}
	}
}

// printInLinux 在linux模式下的打印函数
func (bc *BarConfig) printInLinux(str []byte, percent float32, currPos, total int64) {
	_, _ = fmt.Fprintf(os.Stdout, printModel[bc.printModel], bc.showWay, bc.backColor, bc.frontColor, string(str), percent, currPos, total)
}

// printInWindows windows模式下的打印函数
func (bc *BarConfig) printInWindows(str []byte, percent float32, currPos, total int64) {
	_, _ = fmt.Fprintf(os.Stdout, printModel[bc.printModel], string(str), percent, currPos, total)
}

// Finish 完成处理方法，在完成后会将相关数据归零，以便重新使用
func (b *__bar) Finish() {
	fmt.Printf("\n")
	b.left = 0
	b.right = 50
	b.currentPosition = 0
	b.percent = 0.0
}
