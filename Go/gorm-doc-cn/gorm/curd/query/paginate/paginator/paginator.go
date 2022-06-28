package paginator

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Pagination struct {
	Limit  uint `form:"limit"`
	Offset uint `form:"offset"`
}

func (p *Pagination) Validate() {
	if p.Offset <= 0 {
		p.Offset = 0
	}

	switch {
	case p.Limit > 100:
		p.Limit = 100 // max page size
	case p.Limit <= 0:
		p.Limit = 10 // min page size
	}
}

func Paginate(c *gin.Context) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		var pagination Pagination
		if c.ShouldBindQuery(&pagination) == nil {
			pagination.Validate()
		}

		return db.Offset(int(pagination.Offset)).Limit(int(pagination.Limit))
	}
}
