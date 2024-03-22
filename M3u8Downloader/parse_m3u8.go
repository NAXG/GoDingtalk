package M3u8Downloader

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"regexp"
	"strings"
)

var (
	lineParameterPattern = regexp.MustCompile(`([a-zA-Z-]+)=("[^"]+"|[^",]+)`)
)

type (
	CryptMethod string
)

// parseLines 按行来解析m3u8文件
func parseLines(lines []string) (*M3u8, error) {
	var (
		i       = 0
		lineLen = len(lines)
		m3u8    = &M3u8{}
		key     *Key
		seg     *Segment
	)
	for ; i < lineLen; i++ {
		//TrimSpace返回字符串s的一个片段，去掉Unicode定义的所有前导和尾随空格。
		line := strings.TrimSpace(lines[i])
		if i == 0 {
			if "#EXTM3U" != line {
				return nil, errorMap[InvalidM3u8Exception]
			}
			continue
		}
		switch {
		case line == "":
			continue
		case strings.HasPrefix(line, "#EXT-X-STREAM-INF:"):
			i++
			m3u8.MasterPlaylistURIs = append(m3u8.MasterPlaylistURIs, lines[i])
			continue
		case !strings.HasPrefix(line, "#"):
			seg = new(Segment)
			seg.URI = line
			m3u8.Segments = append(m3u8.Segments, seg)
			seg.Key = key
			continue
		case strings.HasPrefix(line, "#EXT-X-KEY"):
			params := parseLineParameters(line)
			if len(params) == 0 {
				return nil, errorMap[InvalidEXT_X_KEY]
			}
			key = new(Key)
			method := CryptMethod(params["METHOD"])
			if method != "" && method != CryptMethodAES && method != CryptMethodNONE {
				return nil, errorMap[InvalidEXT_X_KEYMethod]
			}
			key.Method = method
			key.URI = params["URI"]
			key.IV = params["IV"]
		default:
			continue
		}
	}
	return m3u8, nil
}

func parseLineParameters(line string) map[string]string {
	r := lineParameterPattern.FindAllStringSubmatch(line, -1)
	params := make(map[string]string)
	for _, arr := range r {
		params[arr[1]] = strings.Trim(arr[2], "\"")
	}
	return params
}

func AES128Encrypt(origData, key, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockSize := block.BlockSize()
	if len(iv) == 0 {
		iv = key
	}
	origData = pkcs5Padding(origData, blockSize)
	blockMode := cipher.NewCBCEncrypter(block, iv[:blockSize])
	crypted := make([]byte, len(origData))
	blockMode.CryptBlocks(crypted, origData)
	return crypted, nil
}

func AES128Decrypt(crypted, key, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockSize := block.BlockSize()
	if len(iv) == 0 {
		iv = key
	}
	blockMode := cipher.NewCBCDecrypter(block, iv[:blockSize])
	origData := make([]byte, len(crypted))
	blockMode.CryptBlocks(origData, crypted)
	origData = pkcs5UnPadding(origData)
	return origData, nil
}

func pkcs5Padding(cipherText []byte, blockSize int) []byte {
	padding := blockSize - len(cipherText)%blockSize
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(cipherText, padText...)
}

func pkcs5UnPadding(origData []byte) []byte {
	length := len(origData)
	unPadding := int(origData[length-1])
	return origData[:(length - unPadding)]
}
