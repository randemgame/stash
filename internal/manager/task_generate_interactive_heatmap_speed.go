package manager

import (
	"context"
	"fmt"
	"time"

	"github.com/stashapp/stash/pkg/file/video"
	"github.com/stashapp/stash/pkg/fsutil"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
)

type GenerateInteractiveHeatmapSpeedTask struct {
	repository          models.Repository
	Scene               models.Scene
	Overwrite           bool
	fileNamingAlgorithm models.HashAlgorithm
}

func (t *GenerateInteractiveHeatmapSpeedTask) GetDescription() string {
	return fmt.Sprintf("Generating heatmap and speed for %s", t.Scene.Path)
}

func (t *GenerateInteractiveHeatmapSpeedTask) Start(ctx context.Context) {
	if !t.required() {
		return
	}

	videoChecksum := t.Scene.GetHash(t.fileNamingAlgorithm)
	funscriptPath := video.GetFunscriptPath(t.Scene.Path)
	heatmapPath := instance.Paths.Scene.GetInteractiveHeatmapPath(videoChecksum)
	drawRange := instance.Config.GetDrawFunscriptHeatmapRange()

	generator := NewInteractiveHeatmapSpeedGenerator(drawRange)

	err := generator.Generate(funscriptPath, heatmapPath, t.Scene.Files.Primary().Duration)

	if err != nil {
		logger.Errorf("error generating heatmap for %s: %s", t.Scene.Path, err.Error())
		return
	}

	median := generator.InteractiveSpeed

	r := t.repository
	if err := r.WithTxn(ctx, func(ctx context.Context) error {
		primaryFile := t.Scene.Files.Primary()
		primaryFile.InteractiveSpeed = &median
		qb := r.File
		return qb.Update(ctx, primaryFile)
	}); err != nil && ctx.Err() == nil {
		logger.Error(err.Error())
	}

	if err := r.WithTxn(ctx, func(ctx context.Context) error {
		qb := r.Scene
		scenePartial := models.NewScenePartial()
		now := time.Now()
		scenePartial.UpdatedAt = models.NewOptionalTime(now)

		_, err := qb.UpdatePartial(ctx, t.Scene.ID, scenePartial)
		return err
	}); err != nil && ctx.Err() == nil {
		logger.Errorf("error updating %s after heatmap generation: %s", t.Scene.Path, err.Error())
	}
}

func (t *GenerateInteractiveHeatmapSpeedTask) required() bool {
	primaryFile := t.Scene.Files.Primary()
	if primaryFile == nil || !primaryFile.Interactive {
		return false
	}

	if t.Overwrite {
		return true
	}

	sceneHash := t.Scene.GetHash(t.fileNamingAlgorithm)
	return !t.doesHeatmapExist(sceneHash) || primaryFile.InteractiveSpeed == nil
}

func (t *GenerateInteractiveHeatmapSpeedTask) doesHeatmapExist(sceneChecksum string) bool {
	if sceneChecksum == "" {
		return false
	}

	imageExists, _ := fsutil.FileExists(instance.Paths.Scene.GetInteractiveHeatmapPath(sceneChecksum))
	return imageExists
}
