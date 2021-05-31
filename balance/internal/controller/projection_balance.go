package controller

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain"
	"github.com/quintans/eventsourcing/eventid"
	"github.com/quintans/eventsourcing/projection"
)

type ProjectionBalance struct {
	projectionUC domain.ProjectionUsecase
	rebuilder    projection.Rebuilder
}

func NewProjectionBalance(
	projectionUC domain.ProjectionUsecase,
	rebuilder projection.Rebuilder,
) ProjectionBalance {
	return ProjectionBalance{
		projectionUC: projectionUC,
		rebuilder:    rebuilder,
	}
}

func (p ProjectionBalance) RebuildBalance(c echo.Context) error {
	var ts time.Time

	epoch := c.QueryParams().Get("epoch")
	if epoch != "" {
		secs, err := strconv.Atoi(epoch)
		if err != nil {
			return c.String(http.StatusBadRequest, err.Error())
		}
		ts = time.Unix(int64(secs), 0)
	}

	ctx := c.Request().Context()
	err := p.rebuilder.Rebuild(
		ctx,
		domain.ProjectionBalance,
		func(ctx context.Context) (eventid.EventID, error) {
			return p.projectionUC.RebuildBalance(ctx, ts)
		},
		p.projectionUC.RebuildWrapUp,
	)
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusOK)
}
