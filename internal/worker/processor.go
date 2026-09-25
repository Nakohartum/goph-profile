package worker

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"

	"github.com/disintegration/imaging"
	_ "golang.org/x/image/webp"

	"goph-profile/internal/domain"
	"goph-profile/internal/ports"
)

type Processor struct {
	repo    ports.AvatarRepository
	storage ports.ObjectStorage
}

func NewProcessor(r ports.AvatarRepository, s ports.ObjectStorage) *Processor {
	return &Processor{r, s}
}
func (p *Processor) Handle(ctx context.Context, e domain.ProcessEvent) error {
	if e.Action == "delete" {
		keys := e.S3Keys
		if len(keys) == 0 {
			keys = []string{e.S3Key}
		}
		for _, k := range keys {
			if err := p.storage.Delete(ctx, k); err != nil {
				return fmt.Errorf("delete %s: %w", k, err)
			}
		}
		return nil
	}
	claimed, err := p.repo.BeginProcessing(ctx, e.AvatarID, e.MessageID)
	if err != nil {
		return err
	}
	if !claimed {
		return nil
	}
	if err = p.process(ctx, e); err != nil {
		_ = p.repo.FailProcessing(ctx, e.AvatarID)
		return err
	}
	return nil
}
func (p *Processor) process(ctx context.Context, e domain.ProcessEvent) error {
	r, _, err := p.storage.Get(ctx, e.S3Key)
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()
	img, _, err := image.Decode(r)
	if err != nil {
		return fmt.Errorf("decode image: %w", err)
	}
	bounds := img.Bounds()
	thumbs := map[string]string{}
	for _, n := range []int{100, 300} {
		resized := imaging.Fill(img, n, n, imaging.Center, imaging.Lanczos)
		var b bytes.Buffer
		if err = jpeg.Encode(&b, resized, &jpeg.Options{Quality: 85}); err != nil {
			return err
		}
		key := fmt.Sprintf("avatars/%s/%dx%d.jpg", e.AvatarID, n, n)
		if err = p.storage.Put(ctx, key, bytes.NewReader(b.Bytes()), int64(b.Len()), "image/jpeg"); err != nil {
			return err
		}
		thumbs[fmt.Sprintf("%dx%d", n, n)] = key
	}
	return p.repo.CompleteProcessing(ctx, e.AvatarID, bounds.Dx(), bounds.Dy(), thumbs)
}
