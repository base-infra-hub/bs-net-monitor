package ip

import (
	"strconv"
	"sync"

	"bs-net-monitor/internal/service"
	"bs-net-monitor/pkg/middleware"
	"bs-net-monitor/pkg/response"

	"github.com/gin-gonic/gin"
)

// ImportHandler 处理从 Excel 导入 IP。
type ImportHandler struct {
	svc *service.IPService
}

var (
	importHandlerInstance *ImportHandler
	importHandlerOnce     sync.Once
)

// GetImportHandler 返回 IP 导入 Handler 的单例。
func GetImportHandler() *ImportHandler {
	importHandlerOnce.Do(func() {
		importHandlerInstance = &ImportHandler{
			svc: service.GetIPService(),
		}
	})
	return importHandlerInstance
}

func (h *ImportHandler) Import(c *gin.Context) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		response.BadRequest(c, "缺少文件")
		return
	}
	defer file.Close()

	tacticsIdStr := c.PostForm("tacticsId")
	tacticsId, err := strconv.ParseUint(tacticsIdStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "tacticsId 无效")
		return
	}

	count, err := h.svc.ImportIPs(middleware.TenantFromContext(c), file, tacticsId)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, gin.H{"imported": count})
}

func (h *ImportHandler) Export(c *gin.Context) {
	tacticsIdStr := c.Query("tacticsId")
	var tacticsId uint64
	if tacticsIdStr != "" {
		var err error
		tacticsId, err = strconv.ParseUint(tacticsIdStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "tacticsId 无效")
			return
		}
	}

	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename=ips_export.xlsx")

	if err := h.svc.ExportIPs(middleware.TenantFromContext(c), tacticsId, c.Writer); err != nil {
		c.Status(500)
		return
	}
}

func (h *ImportHandler) Template(c *gin.Context) {
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename=ip_template.xlsx")

	if err := h.svc.GetImportTemplate(c.Writer); err != nil {
		c.Status(500)
		return
	}
}
