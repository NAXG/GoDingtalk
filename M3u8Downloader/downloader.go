package M3u8Downloader

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	CryptMethodAES        CryptMethod = "AES-128"
	CryptMethodNONE       CryptMethod = "NONE"
	defaultNumberOfThread             = 10
	//WriteIntoCacheAndSaveModel 使用缓存下载模式的处理器函数,暂时废弃，因为会占用大量内存，且视频质量不高
	WriteIntoCacheAndSaveModel = 1
	SaveAsTsFileAndMergeModel  = 2
	SuffixMp4                  = ".ts"
	SuffixTs                   = ".ts"
	TestDownloadUrl            = ""
)

const (
	NoException DownloadExceptionType = iota
	UrlException
	IOException
	NetworkException
	UnexpectedException
	ReadDirectoryException
	InvalidM3u8Exception
	InvalidEXT_X_KEY
	InvalidEXT_X_KEYMethod
	HttpException
	NoM3u8SegmentException
	DecrytTSFailed
)

var (
	defaultSaveDirectory = "./video/"
	downloadModelMap     = map[DownloadModelType]DownloadModelType{}
	errorMap             = map[DownloadExceptionType]error{}
)

// init 初始化函数
func init() {
	downloadModelMap[WriteIntoCacheAndSaveModel] = WriteIntoCacheAndSaveModel
	downloadModelMap[SaveAsTsFileAndMergeModel] = SaveAsTsFileAndMergeModel
	errorMap[UrlException] = errors.New("[URLException]: Invalid URL format, please check your URL")
	errorMap[IOException] = errors.New("[IOException]: Failed to read or write file")
	errorMap[ReadDirectoryException] = errors.New("[ReadDirectoryException]: Failed to read directory")
	errorMap[NetworkException] = errors.New("[NetworkException]: Network connection failed")
	errorMap[InvalidM3u8Exception] = errors.New("[InvalidM3u8Exception]: Invalid M3U8 file format")
	errorMap[InvalidEXT_X_KEY] = errors.New("[InvalidEXT_X_KEY]: Invalid encryption key in M3U8 file")
	errorMap[InvalidEXT_X_KEYMethod] = errors.New("[InvalidEXT_X_KEYMethod]: Unsupported encryption method")
	errorMap[NoM3u8SegmentException] = errors.New("[NoM3u8SegmentException]: No video segments found in M3U8 file")
	errorMap[DecrytTSFailed] = errors.New("[DecryptTSFailed]: Failed to decrypt video segment")
}

// 自定义类型
type (
	IntChannel            chan int
	DownloadExceptionType int
	DownloadModelType     int
	DownloadModelFunction func(int, int, []byte)
)

// M3u8Downloader 下载器接口对象
type M3u8Downloader interface {
	// DefaultDownload 默认下载方式，建议使用，方便快捷,下载为ts文件，然后合并，将会返回一个下载状态
	DefaultDownload() bool
	// ParseM3u8FileEncrypted 解析加密m3u8文件
	ParseM3u8FileEncrypted(link string) (*Result, error)
	// Download 根据之前的配置开始执行下载
	Download() error
	// SetUrl 设置url
	SetUrl(url string)
	// SetIfShowTheBar 是否显示进度条
	SetIfShowTheBar(ifShow bool)
	// SetNumOfThread 设置下载线程的数量
	SetNumOfThread(num int)
	// SetMovieName 设置视频的文件名
	SetMovieName(videoName string)
	// SetSaveDirectory 设置保存目录
	SetSaveDirectory(targetDir string)
	// SetDownloadModel 设置下载模式
	SetDownloadModel(model DownloadModelType)
	// MergeFile 默认合并文件
	MergeFile() error
	// MergeFileInDir 将合并后的视频文件保存到目录dir中
	MergeFileInDir(path string, saveName string) error
}

// m3u8downloader 下载器对象结构体
type m3u8downloader struct {
	// config 下载配置
	config *DownloadConfig
	// taskChannel 发布下载任务的管道
	taskChannel IntChannel
	// suffixList 需要合并的url列表
	suffixList []string
	//waitGroup 用来保证线程同步
	waitGroup *sync.WaitGroup
	// buffer 用于构造一些字符串所需要使用的缓冲区
	buffer []strings.Builder
	// cacheMap 在使用“按顺序写入”下载模式时承载缓存信息
	//建对应着视频的索引，值对应的暂时缓存在内存中等待写入的请求结果对象的[]byte信息
	cacheMap map[int][]byte
	//exception 下载中的异常
	exception DownloadExceptionType
	// m3u8ParseResult 对于m3u8解析的结果
	m3u8ParseResult *Result
	// successChan
	successChan IntChannel
}

// DownloadConfig 下载配置对象
// 因为最后还有很多拼接字符串的地方，所以我们尽可能使用[]byte和StringBuilder
// 以减少不必要的开销
type DownloadConfig struct {
	// Url 下载链接
	Url []byte
	// noSuffixUrl 经过处理后得到无后缀的url
	noSuffixUrl []byte
	// NumOfThreads 下载的线程数
	NumOfThreads int
	// VideoName 保存的视频名称
	VideoName string
	// SaveDirectory 保存视频的目录
	SaveDirectory string
	// ifShowBar 是否在控制台显示进度条
	ifShowBar bool
	// errCount 下载出错的数量
	errCount int
	// errMutex 保护errCount的互斥锁
	errMutex sync.Mutex
	// completeCount 下载完成的数量
	completeCount int
	// TotalNum 总共的下载数量
	TotalNum int
	// DownloadModel 设置下载模式
	DownloadModel DownloadModelType
}

// M3u8 解析m3u8文件后的内容的集合
type M3u8 struct {
	Segments           []*Segment
	MasterPlaylistURIs []string
}

// Segment 包含了.m3u8文件中每一条url
type Segment struct {
	URI string
	Key *Key
}

// Key 对于加密结构体的数据载体
type Key struct {
	URI    string
	IV     string
	key    string
	Method CryptMethod
}

// Result m3u8文件的处理结果对象
type Result struct {
	URL  *url.URL
	M3u8 *M3u8
	Keys map[*Key]string
}

// NewDownloader 创建一个新下载器对象
func NewDownloader() M3u8Downloader {
	return defaultConstructor(&DownloadConfig{
		NumOfThreads:  defaultNumberOfThread,
		ifShowBar:     false,
		SaveDirectory: defaultSaveDirectory,
		Url:           nil,
		DownloadModel: SaveAsTsFileAndMergeModel,
	})
}

// NewDownloaderWithConfig 使用自定义配置创建下载器对象
func NewDownloaderWithConfig(config *DownloadConfig) M3u8Downloader {
	return defaultConstructor(config)
}

// defaultConstructor 默认构造函数
func defaultConstructor(config *DownloadConfig) M3u8Downloader {
	return &m3u8downloader{
		config:      config,
		buffer:      make([]strings.Builder, defaultNumberOfThread),
		cacheMap:    nil,
		suffixList:  nil,
		exception:   NoException,
		waitGroup:   &sync.WaitGroup{},
		taskChannel: nil,
		successChan: nil,
	}
}

// httpGetBodyToByte 以byte数组的形式返回请求体的内容
func (md *m3u8downloader) httpGetBodyToByte(url string) []byte {
	res, exception := httpGet(url)
	if exception != NoException {
		//对异常进行标识，其他线程将会识别到
		md.exception = exception
		return nil
	}
	defer res.Close()
	body, err := io.ReadAll(res)
	if err != nil {
		md.exception = IOException
		return nil
	}
	return body
}

func (md *m3u8downloader) DefaultDownload() bool {
	md.config.DownloadModel = SaveAsTsFileAndMergeModel
	err := md.Download()
	if err != nil {
		fmt.Println(err.Error())
		return false
	}
	err = md.MergeFile()
	if err != nil {
		fmt.Println(err.Error())
		return false
	}
	return true
}

// SetUrl 设置需要下载视频的.m3u8文件的url
func (md *m3u8downloader) SetUrl(url string) {
	//设置url
	md.config.Url = []byte(url)
	//查找url中的最后一个‘/’,并作为noSuffixUrl的相关参数来构造noSuffixUrl
	last := reFind(len(md.config.Url)-1, md.config.Url)
	md.config.noSuffixUrl = make([]byte, last)
	md.config.noSuffixUrl = md.config.Url[0 : last+1]
}

// SetIfShowTheBar 是否显示进度条
func (md *m3u8downloader) SetIfShowTheBar(ifShow bool) {
	md.config.ifShowBar = ifShow
}

// SetNumOfThread 设置线程数量
func (md *m3u8downloader) SetNumOfThread(num int) {
	md.config.NumOfThreads = num
	md.buffer = make([]strings.Builder, num)

}

// SetMovieName 设置保存后的视频名称
func (md *m3u8downloader) SetMovieName(videoName string) {
	if strings.HasSuffix(videoName, SuffixMp4) {
		md.config.VideoName = videoName
	} else {
		md.config.VideoName = videoName + SuffixMp4
	}
}

// SetSaveDirectory 设置下载的视频的保存路径
func (md *m3u8downloader) SetSaveDirectory(targetDir string) {
	temp := targetDir
	if temp[len(temp)-1] != '/' {
		temp += "/"
	}
	md.config.SaveDirectory = temp
}

// SetDownloadModel 设置下载模式
func (md *m3u8downloader) SetDownloadModel(model DownloadModelType) {
	var ok bool
	md.config.DownloadModel, ok = downloadModelMap[model]
	if !ok {
		md.config.DownloadModel = SaveAsTsFileAndMergeModel
	}
}

// showTheBar 显示进度条方法
func (md *m3u8downloader) showTheBar() {
	md.printInfo()
	var total = md.config.TotalNum
	var bar = NewBar(int64(total))
	bar.Setting().SetShowModel(LinuxTerminal)
	var ok bool
	var num int
	bar.Update(0)
	for {
		num, ok = <-md.successChan
		if !ok {
			break
		}
		bar.Update(int64(num))
		//time.Sleep(300*time.Millisecond)
	}
	bar.Finish()

}

// reFind 倒叙查找
func reFind(startIndex int, str []byte) int {
	var i int
	for i = startIndex; str[i] != '/'; i-- {
	}
	return i
}

// printInfo 打印下载信息
func (md *m3u8downloader) printInfo() {
	fmt.Printf("下载任务总数：%d\n", md.config.TotalNum)
	fmt.Printf("下载线程数量：%d\n", md.config.NumOfThreads)
	fmt.Printf("视频保存目录：%s\n", md.config.SaveDirectory)
	fmt.Printf("视频保存名称：%s\n", md.config.VideoName)
	fmt.Printf("视频下载链接：%s\n", md.config.Url)
}

// Download 下载任务核心方法
func (md *m3u8downloader) Download() error {
	var err error
	err = CheckAndCreatDirectory(md.config.SaveDirectory)
	if err != nil {
		return err
	}
	// 首先解析url
	md.m3u8ParseResult, err = md.ParseM3u8FileEncrypted(string(md.config.Url))
	if err != nil {
		return errorMap[UrlException]
	}
	if md.config.VideoName == "" {
		//如果没有设置name,使用时间戳来代替
		md.config.VideoName = getUnixTimeAndToByte()
	}
	//将参数归零
	md.successChan = make(IntChannel, md.config.NumOfThreads)
	md.config.errCount = 0
	md.config.completeCount = 0
	md.config.TotalNum = len(md.m3u8ParseResult.M3u8.Segments)
	md.suffixList = make([]string, md.config.TotalNum)
	md.waitGroup.Add(md.config.NumOfThreads)
	//选择下载模式
	var callBackFunc DownloadModelFunction
	if md.config.DownloadModel == WriteIntoCacheAndSaveModel {
		go md.WriteIntoCacheAndSaveProcessor()
		callBackFunc = md.WriteIntoCacheAndSave
	} else {
		callBackFunc = md.SaveAsTsFileAndMergeEncryption
	}
	md.taskChannel = make(IntChannel, 50)
	//发布下载任务
	go md.publishDownloadTask()
	//开启下载线程
	for i := 0; i < md.config.NumOfThreads; i++ {
		go md.download(i, callBackFunc)
	}
	//显示进度条
	if md.config.ifShowBar {
		go md.showTheBar()
	}
	//阻塞等待
	md.waitGroup.Wait()
	//检查异常
	if md.exception != NoException {
		return errorMap[md.exception]
	}
	close(md.successChan)
	return nil
}

// download 执行任务的主逻辑
func (md *m3u8downloader) download(threadId int, downloadModel func(int, int, []byte)) {
	var index int
	var ok bool
	var body []byte
	var segments *Segment
	for {
		//从管道接收信息
		index, ok = <-md.taskChannel
		if !ok {
			break
		} //拼接路径
		segments = md.m3u8ParseResult.M3u8.Segments[index]
		fullURL := ResolveURL(md.m3u8ParseResult.URL, segments.URI)
		//尝试下载，错误达到一定次数停止下载
		for {
			body = md.httpGetBodyToByte(fullURL)
			// 使用互斥锁保护errCount的读写
			md.config.errMutex.Lock()
			currentErrCount := md.config.errCount
			md.config.errMutex.Unlock()

			if currentErrCount < 2*md.config.NumOfThreads {
				if md.exception == NoException {
					break
				}
				//如果出现范围允许的错误，则重试
				md.config.errMutex.Lock()
				md.config.errCount++
				md.config.errMutex.Unlock()
				md.exception = NoException
			} else {
				//若出现严重错误，则通知其他线程，停止工作
				md.waitGroup.Done()
				return
			}
		}
		// 解密 TS 数据
		if segments.Key != nil {
			var err error
			key := md.m3u8ParseResult.Keys[segments.Key]
			if key != "" {
				body, err = AES128Decrypt(body, []byte(key), []byte(segments.Key.IV))
				if err != nil {
					md.exception = DecrytTSFailed //放置异常类型
					md.config.errMutex.Lock()
					md.config.errCount = 2 * md.config.NumOfThreads //抛出严重异常
					md.config.errMutex.Unlock()
					md.waitGroup.Done()
					return
					//fmt.Printf("decryt TS failed: %s\n", err.Error())
				}
			}
		}
		//处理解密
		syncByte := uint8(71) //0x47
		bLen := len(body)
		for j := 0; j < bLen; j++ {
			if body[j] == syncByte {
				body = body[j:]
				break
			}
		}
		//执行下载回调函数
		downloadModel(index, threadId, body)
	}
	md.waitGroup.Done()
}

// SaveAsTsFileAndMergeEncryption 下载为ts文件，最后手动合并
func (md *m3u8downloader) SaveAsTsFileAndMergeEncryption(index, threadId int, body []byte) {
	md.buffer[threadId].WriteString(md.config.SaveDirectory)
	md.buffer[threadId].Write(processNum(index))
	md.suffixList[index] = md.buffer[threadId].String()
	movie, err := os.OpenFile(md.suffixList[index], os.O_CREATE|os.O_WRONLY, os.ModePerm)
	if err != nil {
		md.config.errMutex.Lock()
		md.config.errCount += md.config.NumOfThreads
		md.config.errMutex.Unlock()
		md.exception = IOException
		return
	}
	movie.Write(body)
	movie.Close()
	md.buffer[threadId].Reset()
	md.config.completeCount++
	md.successChan <- md.config.completeCount
	//fmt.Println(md.config.completeCount)
	body = nil
}

// publishDownloadTask 发布下载任务
func (md *m3u8downloader) publishDownloadTask() {
	for i := 0; i < md.config.TotalNum; i++ {
		md.taskChannel <- i
	}
	close(md.taskChannel)
}

// WriteIntoCacheAndSaveProcessor 使用缓存下载模式的处理器函数,暂时废弃，因为会占用大量内存，且视频质量不高
func (md *m3u8downloader) WriteIntoCacheAndSaveProcessor() {
	var buffer strings.Builder
	buffer.WriteString(md.config.SaveDirectory)
	buffer.WriteString(md.config.VideoName)
	movie, err := os.OpenFile(buffer.String(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, os.ModePerm)
	if err != nil {
		md.config.errMutex.Lock()
		md.config.errCount += md.config.NumOfThreads
		md.config.errMutex.Unlock()
		md.exception = IOException
		return
	}
	defer movie.Close()
	md.cacheMap = map[int][]byte{}
	var body []byte
	var i int
	var ok bool
	//自旋
	for i < md.config.TotalNum {
		//如果字典中有了对应索引的id相关的内容，则追加写入然后继续新的尝试，
		//否则睡眠等待一定时间后再次尝试，以减少资源的消耗，直到成功为止
		if body, ok = md.cacheMap[i]; ok {
			movie.Write(body)
			md.cacheMap[i] = nil
			delete(md.cacheMap, i)

			i++
		} else {
			time.Sleep(250 * time.Millisecond)
		}
	}
	//ReSet The Map And Help GC
	md.cacheMap = nil
}

// WriteIntoCacheAndSave 写入缓存，最后保存
func (md *m3u8downloader) WriteIntoCacheAndSave(index, threadId int, body []byte) {
	md.cacheMap[index] = body
	md.buffer[threadId].Reset()
	md.config.completeCount++
	md.successChan <- md.config.completeCount
}

// MergeFileInDir 将目标目录下的ts文件全部合并
func (md *m3u8downloader) MergeFileInDir(path string, saveName string) error {
	var (
		fileList []string
		err      error
	)
	fileList, err = getAllNonDirectoryFile(path)
	if err != nil {
		return nil
	}
	err = mergeFile(path, fileList, saveName)
	if err != nil {
		return err
	}
	return nil
}

// MergeFile 在下载完毕后文件合并
func (md *m3u8downloader) MergeFile() error {
	if md.suffixList == nil {
		return nil
	}
	err := mergeFile(md.config.SaveDirectory, md.suffixList, md.config.VideoName)
	if err != nil {
		return err
	}
	return nil
}

// parseM3u8FileUnencrypted 解析并处理未加密的m3u8文件
func (md *m3u8downloader) parseM3u8FileUnencrypted(url string) {
	body := md.httpGetBodyToByte(url)
	if md.exception != NoException {
		return
	}
	var temp []byte
	var i, left, bodyLen = 0, 0, len(body) - 1
	for i < bodyLen {
		if body[i] == '/' || body[i] == '\n' {
			i += 1
			left = i
		} else if body[i] == '.' && body[i+1] == 't' {
			i += 3
			temp = body[left:i]
			md.suffixList = append(md.suffixList, string(temp))
		} else {
			i++
		}
	}
}

// ParseM3u8FileEncrypted 解析并处理加密的m3u8文件
func (md *m3u8downloader) ParseM3u8FileEncrypted(link string) (*Result, error) {
	var (
		err       error
		_url      *url.URL
		exception DownloadExceptionType
		body      io.ReadCloser
	)

	_url, err = url.Parse(link)
	if err != nil {
		return nil, err
	}
	body, exception = httpGet(_url.String())
	if exception != NoException {
		return nil, errorMap[exception]
	}
	//noinspection GoUnhandledErrorResult
	defer body.Close()
	s := bufio.NewScanner(body)
	var lines []string
	for s.Scan() {
		lines = append(lines, s.Text())
	}

	//解析请求体内容，m3u8中的内容
	m3u8, err := parseLines(lines)
	if err != nil {
		return nil, err
	}
	// 若为 Master playlist，则再次请求获取 Media playlist
	if m3u8.MasterPlaylistURIs != nil {
		return md.ParseM3u8FileEncrypted(ResolveURL(_url, m3u8.MasterPlaylistURIs[0]))
	}
	if len(m3u8.Segments) == 0 {
		return nil, errorMap[NoM3u8SegmentException]
	}
	result := &Result{
		URL:  _url,
		M3u8: m3u8,
		Keys: make(map[*Key]string),
	}
	// 请求解密秘钥
	var keyByte []byte
	for _, seg := range m3u8.Segments {
		switch {
		case seg.Key == nil || seg.Key.Method == "" || seg.Key.Method == CryptMethodNONE:
			continue
		case seg.Key.Method == CryptMethodAES:
			// 如果已经请求过了，就不在请求
			if _, ok := result.Keys[seg.Key]; ok {
				continue
			}

			keyByte = md.httpGetBodyToByte(ResolveURL(_url, seg.Key.URI))
			if keyByte == nil {
				return nil, errorMap[md.exception]
			}
			result.Keys[seg.Key] = string(keyByte)
		default:
			return nil, fmt.Errorf("[UnexpectedException]unknown or unsupported cryption method: %s", seg.Key.Method)
		}
	}
	return result, nil
}
