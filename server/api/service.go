package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/zfd81/rock/server"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"github.com/zfd81/rock/core"
	"github.com/zfd81/rock/errs"
	"github.com/zfd81/rock/meta"
	"github.com/zfd81/rock/meta/dai"
	"github.com/zfd81/rock/script"
)

func TestAnalysis(c *gin.Context) {
	p, err := param(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errs.NewError(err))
		return
	}
	source := p.GetString("source")
	serv, err := SourceAnalysis(source)
	if err != nil {
		c.JSON(http.StatusBadRequest, err)
		return
	}
	serv.Name = p.GetString("name")
	c.JSON(http.StatusOK, serv)
}

func Test(c *gin.Context) {
	p, err := param(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errs.NewError(err))
		return
	}
	source := p.GetString("source")
	_, code := SplitSource(source)
	serv, err := SourceAnalysis(source)
	if err != nil {
		c.JSON(http.StatusBadRequest, err)
		return
	}
	serv.Name = p.GetString("name")
	serv.Source = code
	res := core.NewResource(serv)
	ps, found := p.Get("params")
	if found {
		params := cast.ToStringMap(ps)
		for _, param := range res.GetPathParams() {
			param.Value = cast.ToString(params[param.Name])
		}
		for _, param := range res.GetRequestParams() {
			val, found := params[param.Name]
			if !found {
				c.JSON(http.StatusBadRequest, errs.New(errs.ErrParamNotFound, param.Name))
				return
			}
			if strings.ToUpper(param.DataType) == meta.DataTypeString {
				param.Value = cast.ToString(val)
			} else if strings.ToUpper(param.DataType) == meta.DataTypeInteger {
				param.Value = cast.ToInt(val)
			} else if strings.ToUpper(param.DataType) == meta.DataTypeBool {
				param.Value = cast.ToBool(val)
			} else if strings.ToUpper(param.DataType) == meta.DataTypeMap {
				param.Value = cast.ToStringMap(val)
			} else if strings.ToUpper(param.DataType) == meta.DataTypeArray {
				param.Value = cast.ToStringSlice(val)
			}
		}
	}
	res.SetContext(server.NewContext(res.GetNamespace()))
	log, resp, err := res.Run()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"log": log,
		})
		return
	}
	for k, v := range resp.Header {
		c.Header(k, v)
	}
	c.JSON(http.StatusOK, gin.H{
		"log":    log,
		"header": resp.Header,
		"data":   resp.Data,
	})
}

func CreateService(c *gin.Context) {
	p, err := param(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errs.NewError(err))
		return
	}
	serv, err := SourceAnalysis(p.GetString("source"))
	if err != nil {
		c.JSON(http.StatusBadRequest, err)
		return
	}
	serv.Name = p.GetString("name")
	serv.Source = p.GetString("source")
	err = dai.CreateService(serv)
	if err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"msg": fmt.Sprintf("Service %s created successfully", serv.Path),
	})
}

func DeleteService(c *gin.Context) {
	namespace := c.Request.Header.Get("namespace") //从Header中获得命名空间
	method := c.Param("method")
	m := strings.ToUpper(method)
	if m != http.MethodGet &&
		m != http.MethodPost &&
		m != http.MethodPut &&
		m != http.MethodDelete {
		c.JSON(http.StatusBadRequest, errs.New(errs.ErrParamBad, "Method "+method+" not found"))
		return
	}
	path := c.Param("path")
	serv := &meta.Service{
		Namespace: namespace,
		Method:    method,
		Path:      path,
	}
	err := dai.DeleteService(serv)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errs.NewError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"msg": fmt.Sprintf("Service %s deleted successfully", serv.Path),
	})
}

func ModifyService(c *gin.Context) {
	p, err := param(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errs.NewError(err))
		return
	}
	serv, err := SourceAnalysis(p.GetString("source"))
	if err != nil {
		c.JSON(http.StatusBadRequest, err)
		return
	}
	serv.Source = p.GetString("source")
	err = dai.ModifyService(serv)
	if err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"msg": fmt.Sprintf("Service %s modified successfully", serv.Path),
	})
}

func FindService(c *gin.Context) {
	namespace := c.Request.Header.Get("namespace") //从Header中获得命名空间
	method := c.Param("method")
	m := strings.ToUpper(method)
	if m != http.MethodGet &&
		m != http.MethodPost &&
		m != http.MethodPut &&
		m != http.MethodDelete {
		c.JSON(http.StatusBadRequest, errs.New(errs.ErrParamBad, "Method "+method+" not found"))
		return
	}
	path := c.Param("path")
	serv, err := dai.GetService(namespace, m, path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errs.NewError(err))
		return
	}
	c.JSON(http.StatusOK, serv)
}

func ListService(c *gin.Context) {
	namespace := c.Request.Header.Get("namespace") //从Header中获得命名空间
	path := c.Param("path")
	servs, err := dai.ListService(namespace, path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errs.NewError(err))
		return
	}

	paths := make([]string, 0, 50)
	for _, serv := range servs {
		paths = append(paths, strings.ToUpper(serv.Method)+":"+serv.Path)
	}

	c.JSON(http.StatusOK, paths)
}

func SplitSource(source string) (string, string) {
	start := strings.Index(source, "$.define(")
	if start == -1 {
		return "", source
	}
	end := strings.Index(source[start:], "})")
	if end == -1 {
		return "", source
	}
	return source[start : end+3], source[end+3:]
}

func wrapService(definition string, serv *meta.Service) error {
	if definition == "" {
		return errs.New(errs.ErrParamBad, "Missing service definition")
	}
	se := script.New(nil)
	se.SetScript(definition)
	se.Run()
	data, err := se.GetVar("__serv_definition")
	if err != nil {
		return errs.New(errs.ErrParamBad, "Service definition error")
	}
	val, ok := data.(map[string]interface{})
	if !ok {
		return errs.New(errs.ErrParamBad, "Service definition error")
	}
	namespace := cast.ToString(val["namespace"])
	path := cast.ToString(val["path"])
	if path == "" {
		return errs.New(errs.ErrParamBad, "Service path not found")
	}
	method := cast.ToString(val["method"])
	if method == "" {
		return errs.New(errs.ErrParamBad, "Service method not found")
	}
	m := strings.ToUpper(method)
	if m != http.MethodGet &&
		m != http.MethodPost &&
		m != http.MethodPut &&
		m != http.MethodDelete {
		return errs.New(errs.ErrParamBad, "Service method["+method+"] error")
	}
	params := val["params"]
	serv.Namespace = namespace
	serv.Path = path
	serv.Method = method
	if params != nil {
		ps, ok := params.([]map[string]interface{})
		if !ok {
			return errs.New(errs.ErrParamBad, "Service parameter definition error")
		}
		for _, param := range ps {
			serv.AddParam(cast.ToString(param["name"]), cast.ToString(param["dataType"]))
		}
	}
	return nil
}

func SourceAnalysis(source string) (*meta.Service, error) {
	definition, code := SplitSource(source)
	serv := &meta.Service{}
	se := script.New(nil)
	var namespace string
	var path string
	var method string
	if definition != "" {
		se.SetScript(definition)
		se.Run()
		data, err := se.GetVar("__serv_definition")
		if err != nil {
			return nil, errs.New(errs.ErrParamBad, "Service definition error")
		}
		val, ok := data.(map[string]interface{})
		if !ok {
			return nil, errs.New(errs.ErrParamBad, "Service definition error")
		}
		namespace = cast.ToString(val["namespace"])
		path = cast.ToString(val["path"])
		if path == "" {
			return nil, errs.New(errs.ErrParamBad, "Service path not found")
		}
		method = cast.ToString(val["method"])
		if method == "" {
			return nil, errs.New(errs.ErrParamBad, "Service method not found")
		}
		m := strings.ToUpper(method)
		if m != http.MethodGet && m != http.MethodPost &&
			m != http.MethodPut && m != http.MethodDelete {
			return nil, errs.New(errs.ErrParamBad, "Service method["+method+"] error")
		}
		params := val["params"]
		if params != nil {
			ps, ok := params.([]map[string]interface{})
			if !ok {
				return nil, errs.New(errs.ErrParamBad, "Service parameter definition error")
			}
			for _, param := range ps {
				serv.AddParam(cast.ToString(param["name"]), cast.ToString(param["dataType"]))
			}
		}
	} else {
		se.AddScript("var module={};")
		se.AddScript(code)
		err := se.Run()
		if err != nil {
			return nil, err
		}
		value, err := se.GetVar("module")
		if err != nil {
			return nil, err
		}
		module, ok := value.(map[string]interface{})
		if ok {
			value = module["exports"]
			exports, ok := value.(map[string]interface{})
			if !ok {
				return nil, errs.New(errs.ErrParamBad, "Module definition error")
			}
			namespace = cast.ToString(exports["namespace"])
			path = cast.ToString(exports["path"])
			if path == "" {
				return nil, errs.New(errs.ErrParamBad, "Module path not found")
			}
			method = "LOCAL"
		}
	}
	serv.Namespace = namespace
	serv.Path = path
	serv.Method = method
	return serv, nil
}