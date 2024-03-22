package M3u8Downloader

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
	"unsafe"
)

// processNum 将小于10000的数字转化为字符串(前面补0)并添加后缀
func processNum(n int) []byte {
	if n < 10 {
		return []byte{48, 48, 48, byte(48 + n), '.', 't', 's'}
	} else if n < 100 {
		return []byte{48, 48, byte(48 + (n / 10)), byte(48 + (n % 10)), '.', 't', 's'}
	} else if n < 1000 {
		return []byte{48, byte(48 + (n / 100)), byte(48 + ((n / 10) % 10)), byte(48 + (n % 10)), '.', 't', 's'}
	} else {
		return []byte{byte(48 + (n / 1000)), byte(48 + ((n / 100) % 10)), byte(48 + ((n / 10) % 10)), byte(48 + (n % 10)), '.', 't', 's'}
	}
}

// getAllNonDirectoryFile 获取所有非目录文件
func getAllNonDirectoryFile(pathName string) ([]string, error) {
	rd, err := ioutil.ReadDir(pathName)
	if err != nil {
		return nil, errorMap[ReadDirectoryException]
	}
	Files := make([]string, 0)
	for i := 0; i < len(rd); i++ {
		if !rd[i].IsDir() {
			fullName := pathName + "/" + rd[i].Name()
			Files = append(Files, fullName)
		}
	}
	rd = nil
	return Files, nil
}

// httpGet 发起get请求
func httpGet(url string) (io.ReadCloser, DownloadExceptionType) {
	client := http.Client{
		Timeout: 30 * time.Second,
	}
	resp, err := client.Get(url)
	if err != nil {
		if err.Error()[len(err.Error())-12:] == "no such host" {
			return nil, NetworkException
		} else {
			errorMap[UnexpectedException] = err
		}
		return nil, UnexpectedException
	}
	if resp.StatusCode != 200 {
		errorMap[HttpException] = fmt.Errorf("[HttpError]:status code %d", resp.StatusCode)
		return nil, HttpException
	}
	return resp.Body, NoException
}

// mergeFile 合并文件主函数
func mergeFile(path string, fileList []string, saveName string) error {
	var (
		buffer strings.Builder
		err    error
		movie  *os.File
	)
	buffer.WriteString(path)
	buffer.WriteString(saveName)
	movie, err = os.OpenFile(buffer.String(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, os.ModePerm)
	if err != nil {
		return err
	}
	defer movie.Close()
	var (
		tsFile *os.File
		body   []byte
	)
	for i := 0; i < len(fileList); i++ {
		tsFile, err = os.OpenFile(fileList[i], os.O_CREATE|os.O_RDONLY, os.ModePerm)
		if err != nil {
			return err
		}
		body, err = ioutil.ReadAll(tsFile)
		if err != nil {
			return err
		}
		_, err = movie.Write(body)
		if err != nil {
			return err
		}
		tsFile.Close()
		err = os.Remove(fileList[i])
		if err != nil {
			return err
		}
	}
	return nil
}

// getUnixTimeAndToByte 根据加当前时间戳设置为默认名称
func getUnixTimeAndToByte() string {
	//将int64转化成string
	t1 := time.Now().Unix()
	var temp int64
	var buf = make([]byte, 10)
	var i = 9
	for t1 > 0 {
		temp = t1 % 10
		buf[i] = (*(*byte)(unsafe.Pointer(&temp))) + 48
		i--
		t1 /= 10
	}
	return *(*string)(unsafe.Pointer(&buf))
}

// ResolveURL 处理Url
func ResolveURL(u *url.URL, p string) string {
	if strings.HasPrefix(p, "https://") || strings.HasPrefix(p, "http://") {
		return p
	}
	var baseURL string
	if strings.Index(p, "/") == 0 {
		baseURL = u.Scheme + "://" + u.Host
	} else {
		tU := u.String()
		baseURL = tU[0:strings.LastIndex(tU, "/")]
	}
	return baseURL + path.Join("/", p)
}

// PathExists 判断目录是否存在，若不存在则创建
func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// CheckAndCreatDirectory 判断目录是否存在，若不存在则创建
func CheckAndCreatDirectory(path string) error {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			//说明文件夹不存在
			err = os.Mkdir(path, os.ModePerm)
			if err != nil {
				return err
			}
			return nil
		} else {
			//说明出现了错误，不能确定文件夹是否存在
			return err
		}
	}
	return nil
}
