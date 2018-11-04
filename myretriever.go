package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"syscall"
	"time"

	"github.com/umeryu/go/sftputil"
)

// ============================================================================
// GLOBAL VARIABLES
// ============================================================================
const TOOLINFO_FILE string = "./myretriever.conf"

// Variables used in TOOL ex. base directory fo retrieve
var toolInfo ToolInfo

//var infos map [string] FileInfo
var infos []FileInfo

// --------------------------------
// retrive file information
// --------------------------------
type FileInfo struct {
	HashID   string    `json:"hashid"` //変数名が公開(大文字でないと）unmarshalできない
	FullPath string    `json:"fullpath"`
	FileDate time.Time `json:"filedate"`
}

// --------------------------------
// tool setting information
// --------------------------------
type ToolInfo struct {
	OUTPUT_DATADIR         string            `json:"output_datadir"`
	RETRIEVE_BASEDIR       string            `json:"retrieve_basedir"`
	RETRIEVE_PRE_SUFFIXIES []PRE_SUFFIX_Info `json:"retrieve_pre_suffixies"`
	OMIT_DIR               []string          `json:"omit_dir"`
	FTP_ANABLE             bool              `json:"ftp_enable"`
	FTP_SITEDIR            string            `json:"ftp_sitedir"`
	FTP_USERINFO           sftputil.UserInfo `json:"ftp_userinfo"`
}

type PRE_SUFFIX_Info struct {
	PREFIX string `json:"prefix"`
	SUFFIX string `json:"suffix"`
}

// ============================================================================
// utilities
// ============================================================================
func toHash(password string) string {
	converted := sha256.Sum256([]byte(password))
	return hex.EncodeToString(converted[:])
}

//----------------------------------------------------
// file manipurate
//----------------------------------------------------
func deleteDir(dirName string) {
	fmt.Println("delete")
	if err := os.RemoveAll(dirName); err != nil {
		fmt.Println(err)
	}
}
func createDir(dirName string) {
	fmt.Println("creat")
	if err := os.MkdirAll(dirName, 0777); err != nil {
		fmt.Println(err)
	}
}

func copy(srcName, dstName string) {

	src, err := os.Open(srcName)
	if err != nil {
		panic(err)
	}
	defer src.Close()

	dst, err := os.Create(dstName)
	if err != nil {
		panic(err)
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	if err != nil {
		panic(err)
	}
}

func listFiles(rootPath, searchPath string) {
	fis, err := ioutil.ReadDir(searchPath)

	if err != nil {
		panic(err)
	}

	var pre_suffixes []PRE_SUFFIX_Info = toolInfo.RETRIEVE_PRE_SUFFIXIES

	for _, fi := range fis {
		fName := fi.Name()

		FullPath := filepath.Join(searchPath, fName)
		if fi.IsDir() {
			if isRetDirectory(fName) {
				//fmt.Println("-->" + FullPath) //DEBUG
				listFiles(rootPath, FullPath)
			}
		} else {
			for i := 0; i < len(pre_suffixes); i++ {
				if isRetFile(fName, pre_suffixes[i].PREFIX, pre_suffixes[i].SUFFIX) {
					addList(FullPath, fName)
				}
			}
		}

	}

}

func addList(FullPath string, fName string) {
	var finfo FileInfo
	var s syscall.Stat_t
	syscall.Stat(FullPath, &s)
	finfo.FullPath = FullPath
	finfo.FileDate = time.Unix(s.Atimespec.Sec, s.Atimespec.Nsec)
	finfo.HashID = toHash(FullPath)
	//infos[fName]=finfo
	infos = append(infos, finfo)
	destPath := filepath.Join(toolInfo.OUTPUT_DATADIR, finfo.HashID)
	copy(finfo.FullPath, destPath)
}

func isRetDirectory(fName string) (b bool) {

	for i := 0; i < len(toolInfo.OMIT_DIR); i++ {
		if fName == toolInfo.OMIT_DIR[i] {
			return false
		}
	}

	return true
}

func isRetFile(fName string, prefix string, suffix string) (b bool) {
	re, _ := regexp.Compile("^" + prefix + "(.*)" + suffix + "$")
	if m := re.MatchString(fName); !m {
		return false
	}
	return true
}

// ============================================================================
// main
// ============================================================================

func init_conf() {
	infos = make([]FileInfo, 0)
	//-----------------------------
	// Load tool setting
	//-----------------------------

	jsonStr, err := ioutil.ReadFile(TOOLINFO_FILE)
	if err != nil {
		fmt.Printf("error:%s¥n", err)
		return
	}
	jsonbytes := ([]byte)(jsonStr)

	err = json.Unmarshal(jsonbytes, &toolInfo)
	if err != nil {
		fmt.Printf("error:%s¥n", err)
		return
	}

	fmt.Println(toolInfo.RETRIEVE_BASEDIR)
	deleteDir(toolInfo.OUTPUT_DATADIR)
	createDir(toolInfo.OUTPUT_DATADIR)
}

func main() {
	fmt.Println("### RETRIEVE TOOL STARD ###")
	init_conf()
	listFiles(toolInfo.RETRIEVE_BASEDIR, toolInfo.RETRIEVE_BASEDIR)

	//--------------------------------
	// create index and output to json file
	//--------------------------------
	jsonbytes2, err2 := json.Marshal(infos)
	if err2 != nil {
		fmt.Printf("error:%s¥n", err2)
		return
	}

	// fmt.Println(string(jsonbytes2))  //DEBUG
	outPath := filepath.Join(toolInfo.OUTPUT_DATADIR, "index.info")
	err := ioutil.WriteFile(outPath, jsonbytes2, os.ModePerm)
	if err != nil {
		fmt.Printf("error:%s¥n", err)
		return
	}

	if toolInfo.FTP_ANABLE == true {
		//--------------------------------
		// load user info json to connect
		//--------------------------------
		var info sftputil.UserInfo = toolInfo.FTP_USERINFO

		fmt.Println(info.Url)
		//--------------------------------
		// file sftp to web site
		//--------------------------------
		// connect by ssh and create sftp client
		var ftpobj sftputil.FileTransport
		ftpobj.Connect(info)
		//fmt.Println(ftpobj)
		var oName string

		for _, f := range infos {
			fmt.Println("---")
			fmt.Println(f.FullPath)
			fmt.Println(f.HashID)
			fmt.Println(f.FileDate)
			oName = toolInfo.FTP_SITEDIR + f.HashID + ".md"
			fmt.Println(oName)
			// put files
			ftpobj.Put(f.FullPath, oName)

		}
		oName = toolInfo.FTP_SITEDIR + "index.json"
		ftpobj.Put(outPath, oName)

		// colse client
		ftpobj.Wrapup()
	}
	fmt.Println("### RETRIEVE TOOL COMPLETED ###")
}
