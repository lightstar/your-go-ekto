package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/lightstar/your-go-ekto/internal/model"

	"github.com/google/uuid"
)

const (
	entitiesDir = "entities"
	evidenceDir = "evidence"
	dirPerm     = 0755
)

// Storage - хранилище сущностей и улик. Сохранение происходит в файловой системе внутри заданной
// директории.
type Storage struct {
	entitiesDir string
	evidenceDir string
}

// New создает новое хранилище, автоматически создавая все нужные директории.
func New(rootDir string) (*Storage, error) {
	if rootDir == "" {
		return nil, ErrNoRootDir
	}

	entitiesDir := filepath.Join(rootDir, entitiesDir)
	evidenceDir := filepath.Join(rootDir, evidenceDir)

	if err := os.MkdirAll(entitiesDir, dirPerm); err != nil {
		return nil, storageOpError("create entities dir", err)
	}

	if err := os.MkdirAll(evidenceDir, dirPerm); err != nil {
		return nil, storageOpError("create evidence dir", err)
	}

	return &Storage{
		entitiesDir: entitiesDir,
		evidenceDir: evidenceDir,
	}, nil
}

// GetEntity возвращает сущность по ID ее досье.
func (s *Storage) GetEntity(ctx context.Context, dossierID uuid.UUID) (model.Entity, error) {
	if err := ctx.Err(); err != nil {
		return model.Entity{}, err
	}

	file, err := os.Open(s.entityFilePath(dossierID))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.Entity{}, ErrEntityNotExists
		}
		return model.Entity{}, storageOpError("open file", err)
	}
	defer file.Close()

	var entity model.Entity

	if err := json.NewDecoder(file).Decode(&entity); err != nil {
		return model.Entity{}, storageOpError("decode entity", err)
	}

	return entity, nil
}

// SaveEntity сохраняет сущность в хранилище.
func (s *Storage) SaveEntity(ctx context.Context, entity model.Entity) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}

	entityJSON, err := json.Marshal(entity)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	_, err = s.saveToFile(bytes.NewReader(entityJSON), s.entityFilePath(entity.DossierID), 0)
	return err
}

// RemoveEntity удаляет сущность из хранилища (только файл с JSON, улики нужно удалять отдельно).
func (s *Storage) RemoveEntity(ctx context.Context, dossierID uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := os.Remove(s.entityFilePath(dossierID)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return storageOpError("remove file", err)
	}
	return nil
}

// SaveEvidence сохраняет файл-изображение улики в хранилище.
// Сохранение происходит потоково из источника src с указанным именем файла.
// Если maxSize > 0, то дополнительно ограничивается размер сохраняемых данных, и если он превышен,
// будет возвращена ошибка ErrLarge.
// При ошибке чтения из источника будет возвращена ошибка ErrRead.
// Сохранено при ошибках ничего не будет (временный файл удаляется).
func (s *Storage) SaveEvidence(
	ctx context.Context, src io.Reader, name string, maxSize int64,
) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	if !s.validEvidenceName(name) {
		return 0, ErrInvalidEvidenceName
	}

	wrappedSrc := &sourceReaderWrapper{inner: src}
	return s.saveToFile(wrappedSrc, s.evidenceFilePath(name), maxSize)
}

// RemoveEvidence удаляет файл с уликой из хранилища.
func (s *Storage) RemoveEvidence(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if !s.validEvidenceName(name) {
		return ErrInvalidEvidenceName
	}

	if err := os.Remove(s.evidenceFilePath(name)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return storageOpError("remove file", err)
	}
	return nil
}

// GetEvidence возвращает файл-изображение улики из хранилища в виде времени модификации и
// потока данных. Это полезно для отдачи содержимого с помощью http.ServeContent.
func (s *Storage) GetEvidence(
	ctx context.Context, name string,
) (time.Time, io.ReadSeekCloser, error) {
	if err := ctx.Err(); err != nil {
		return time.Time{}, nil, err
	}

	if !s.validEvidenceName(name) {
		return time.Time{}, nil, ErrInvalidEvidenceName
	}

	file, err := os.Open(s.evidenceFilePath(name))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return time.Time{}, nil, ErrEvidenceNotExists
		}
		return time.Time{}, nil, storageOpError("open file", err)
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		return time.Time{}, nil, storageOpError("file stat", err)
	}

	if !info.Mode().IsRegular() {
		file.Close()
		return time.Time{}, nil, ErrNotARegularFile
	}

	return info.ModTime(), file, nil
}

// saveToFile сохраняет данные из источника src в файл по указанному пути path.
// Если maxSize > 0, то дополнительно ограничивается размер сохраняемых данных, и если он превышен,
// будет возвращена ошибка ErrSourceTooLarge, сохранено при этом ничего не будет.
// Данные сперва сохраняются во временный файл, который при успехе переименовывается в реальный,
// а при любой ошибке - удаляется.
func (s *Storage) saveToFile(src io.Reader, path string, maxSize int64) (size int64, err error) {
	file, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp")
	if err != nil {
		return 0, storageOpError("create file", err)
	}

	defer func() {
		// Если произошла паника, мы не должны переменовывать временный файл.
		// Нужно его, наоборот, удалить, и пробросить панику дальше.
		panicErr := recover()

		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, storageOpError("close file", closeErr))
		}

		if err == nil && panicErr == nil {
			if renameErr := os.Rename(file.Name(), path); renameErr != nil {
				err = storageOpError("rename temp file", renameErr)
			}
		}

		if err != nil || panicErr != nil {
			if removeErr := os.Remove(file.Name()); removeErr != nil {
				err = errors.Join(err, storageOpError("remove temp file", removeErr))
			}
		}

		if panicErr != nil {
			panic(panicErr)
		}
	}()

	if maxSize > 0 && maxSize < math.MaxInt64 {
		src = io.LimitReader(src, maxSize+1)
	}

	written, err := io.Copy(file, src)
	if err != nil {
		if errors.Is(err, ErrRead) {
			return 0, err
		}
		return 0, storageOpError("write to file", err)
	}

	if maxSize > 0 && written > maxSize {
		return 0, sourceError(ErrTooLarge)
	}

	if err := file.Sync(); err != nil {
		return 0, storageOpError("sync file", err)
	}

	return written, nil
}

// validEvidenceName проверяет, что имя файла-улики соответствует требованиям -
// оно не может быть пустым, зарезервированным идентификатором в файловой системе, а также
// не может содержать поддиректории, только имя файла.
func (s *Storage) validEvidenceName(name string) bool {
	return name != "" && name != "." && name != ".." &&
		filepath.IsLocal(name) && filepath.Base(name) == name
}

// entityFilePath формирует полный путь к файлу сущности по идентификатору досье.
func (s *Storage) entityFilePath(dossierID uuid.UUID) string {
	return filepath.Join(s.entitiesDir, dossierID.String()+".json")
}

// evidenceFilePath формирует полный путь к файлу-улике по имени.
func (s *Storage) evidenceFilePath(name string) string {
	return filepath.Join(s.evidenceDir, name)
}

// sourceReaderWrapper оборачивает поток с исходными данными для сохранения, чтобы отлавливать
// ошибки чтения и присоединять к ним ошибку ErrSource с cause == ErrRead.
// Необходимо, т.к. хранилище сохраняет данные потоково с помощью io.Copy, по ошибке которой иначе
// сложно понять, была эта ошибка самого хранилища или чтения из источника данных.
type sourceReaderWrapper struct {
	inner io.Reader
}

func (r *sourceReaderWrapper) Read(p []byte) (int, error) {
	n, err := r.inner.Read(p)
	if err != nil && err != io.EOF {
		return n, fmt.Errorf("%w: %w", sourceError(ErrRead), err)
	}

	return n, err
}
