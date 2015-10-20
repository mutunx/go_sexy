// config
package conf

import (
	"log"
	"strings"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/url"
	"regexp"
	"bytes"
)

type MatchExp struct{
	Exp *regexp.Regexp
	Match int
	Folder interface{} //可选值url,title,none,正则表达式
}

type Config struct {
	Root         *url.URL
	ImageRegex   []*MatchExp
	PageRegex    []*regexp.Regexp
	ImgPageRegex []*regexp.Regexp
	HrefRegex	 []*MatchExp
}

func (c *Config) Load(file string) error {
	content, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}
	
	content = bytes.Replace(content, []byte("\\"), []byte("\\\\"), -1)
	content = bytes.Replace(content, []byte("\\\\\""), []byte("\\\""), -1)
	
	comRegex := regexp.MustCompile(`\s*##.*`)
	content = comRegex.ReplaceAll(content, []byte{})
	
	nRegex := regexp.MustCompile(`\n|\t|\r`)
	content = nRegex.ReplaceAll(content, []byte{})
	
	jsonObj := make(map[string]interface{})
	err = json.Unmarshal(content, &jsonObj)
	if err != nil {
		log.Println(string(content))
		return errors.New("[1]配置文件格式有误:"+err.Error()) 
	}
	
	root := jsonObj["root"].(string)
	temp := strings.ToLower(root)
	if !strings.HasPrefix(temp, "http://") && !strings.HasPrefix(temp, "https://") {
		root = "http://"+root
	}

	c.Root, err = url.Parse(root)
	if err != nil {
		return err
	}

	reg, ok := jsonObj["regex"].(map[string]interface{})
	if !ok {
		return errors.New("[2]解析regex时出错")
	}

	imgRegs, ok := reg["image"].([]interface{})
	if ok {
		c.ImageRegex = make([]*MatchExp, len(imgRegs))
		for i, val := range imgRegs {		
			obj, ok := val.(map[string]interface{})
			if !ok {
				return errors.New("[3]解析regex.image时出错")
			}
			
			exp := obj["exp"].(string)
			c.ImageRegex[i] = &MatchExp{}
			c.ImageRegex[i].Match = int(obj["match"].(float64))
			
			folder := strings.ToLower(obj["folder"].(string))
			if folder != "none" && folder != "url" && folder != "title" {
				c.ImageRegex[i].Folder, err = regexp.Compile(folder)
				if err != nil {
					return errors.New("[4]解析正则表达式" + folder + "时出错")
				}
			}else{
				c.ImageRegex[i].Folder = folder
			}
			
			
			c.ImageRegex[i].Exp, err = regexp.Compile(exp)
			if err != nil {
				return errors.New("[5]解析正则表达式" + exp + "时出错")
			}
		}
	} else {
		return errors.New("[6]解析regex.image时出错")
	}

	pageRegs, ok := reg["page"].([]interface{})
	if ok {		
		c.PageRegex = make([]*regexp.Regexp, len(pageRegs))
		for i, val := range pageRegs {
			valStr := val.(string)
			c.PageRegex[i], err = regexp.Compile(valStr)
			if err != nil {
				return errors.New("[7]解析正则表达式" + valStr + "时出错")
			}
		}
	} else {
		return errors.New("[8]解析regex.page时出错")
	}

	imgPageRegs, ok := reg["imgInPage"].([]interface{})
	if ok {
		c.ImgPageRegex = make([]*regexp.Regexp, len(imgPageRegs))
		for i, val := range imgPageRegs {
			valStr := val.(string)
			c.ImgPageRegex[i], err = regexp.Compile(valStr)
			if err != nil {
				return errors.New("[9]解析正则表达式" + valStr + "时出错")
			}
		}
	} else {
		return errors.New("[10]解析regex.imgInPage时出错")
	}
	
	hrefRegex, ok := reg["href"].([]interface{})
	if ok {
		c.HrefRegex = make([]*MatchExp, len(hrefRegex))
		for i, val := range hrefRegex {
			obj, ok := val.(map[string]interface{})
			if !ok {
				return errors.New("[11]解析regex.href时出错")
			}
			
			exp := obj["exp"].(string)
			
			c.HrefRegex[i] = &MatchExp{}
			c.HrefRegex[i].Match = int(obj["match"].(float64))
			c.HrefRegex[i].Exp, err = regexp.Compile(exp)		
			if err != nil {
				return errors.New("[12]解析正则表达式" + exp + "时出错")
			}
		}
	} else {
		return errors.New("[13]解析regex.hrefRegex时出错")
	}

	return nil
}
