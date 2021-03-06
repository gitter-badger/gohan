package client

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/cloudwan/gohan/util"
	"github.com/olekukonko/tablewriter"
)

var errorKey = "error"

func (gohanClientCLI *GohanClientCLI) formatOutput(rawResult interface{}) string {
	if rawResult == nil {
		return ""
	}
	switch gohanClientCLI.opts.outputFormat {
	case outputFormatTable:
		return gohanClientCLI.formatOutputTable(rawResult)
	default:
		result, _ := json.MarshalIndent(rawResult, "", "\t")
		return fmt.Sprintf("%s", result)
	}
}

func (gohanClientCLI *GohanClientCLI) formatOutputTable(rawResult interface{}) string {
	buffer := bytes.NewBufferString("")
	for k, v := range rawResult.(map[string]interface{}) {
		if k == errorKey {
			return fmt.Sprintf("%v", v)
		}
		switch v.(type) {
		case []interface{}:
			gohanClientCLI.createResourcesTable(buffer, v.([]interface{}))
		case map[string]interface{}:
			gohanClientCLI.createSingleResourceTable(buffer, v.(map[string]interface{}))
		default:
			return fmt.Sprintf("%v", v)
		}
	}
	return buffer.String()
}

func (gohanClientCLI *GohanClientCLI) createResourcesTable(buffer *bytes.Buffer, resources []interface{}) {
	table := tablewriter.NewWriter(buffer)
	allKeysResource := map[string]interface{}{}
	for _, resource := range resources {
		for key := range resource.(map[string]interface{}) {
			allKeysResource[key] = ""
		}
	}
	keys := util.GetSortedKeys(allKeysResource)
	if len(keys) == 0 {
		return
	}
	table.SetHeader(keys)
	for _, resource := range resources {
		resourceSlice := []string{}
		for _, key := range keys {
			v := ""
			if val, ok := resource.(map[string]interface{})[key]; ok {
				v = fmt.Sprint(val)
			}
			resourceSlice = append(resourceSlice, v)
		}
		table.Append(resourceSlice)
	}
	table.Render()
}

func (gohanClientCLI *GohanClientCLI) createSingleResourceTable(buffer *bytes.Buffer, resource map[string]interface{}) {
	table := tablewriter.NewWriter(buffer)
	table.SetHeader([]string{"Property", "Value"})
	keys := util.GetSortedKeys(resource)
	for _, key := range keys {
		table.Append([]string{key, fmt.Sprint(resource[key])})
	}
	table.Render()
}
