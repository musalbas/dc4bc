package client

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/syndtr/goleveldb/leveldb"
)

const (
	offsetKey     = "offset"
	operationsKey = "operations"
)

type State interface {
	SaveOffset(uint64) error
	LoadOffset() (uint64, error)

	SaveFSM(interface{}) error
	LoadFSM() (interface{}, error)

	PutOperation(operation *Operation) error
	DeleteOperation(operationID string) error
	GetOperations() (map[string]*Operation, error)
	GetOperationByID(operationID string) (*Operation, error)
}

type LevelDBState struct {
	sync.Mutex
	stateDb *leveldb.DB
}

func NewLevelDBState(stateDbPath string) (State, error) {
	db, err := leveldb.OpenFile(stateDbPath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open stateDB: %w", err)
	}

	state := &LevelDBState{
		stateDb: db,
	}

	if err := state.initKey(operationsKey, map[string]*Operation{}); err != nil {
		return nil, fmt.Errorf("failed to init %s storage: %w", operationsKey, err)
	}

	return state, nil
}

func (s *LevelDBState) initKey(key string, data interface{}) error {
	if _, err := s.stateDb.Get([]byte(key), nil); err != nil {
		operationsBz, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("failed to marshal storage structure: %w", err)
		}
		err = s.stateDb.Put([]byte(key), operationsBz, nil)
		if err != nil {
			return fmt.Errorf("failed to init state: %w", err)
		}
	}

	return nil
}

func (s *LevelDBState) SaveOffset(offset uint64) error {
	bz := make([]byte, 8)
	binary.LittleEndian.PutUint64(bz, offset)

	if err := s.stateDb.Put([]byte(offsetKey), bz, nil); err != nil {
		return fmt.Errorf("failed to set offset: %w", err)
	}

	return nil
}

func (s *LevelDBState) LoadOffset() (uint64, error) {
	bz, err := s.stateDb.Get([]byte(offsetKey), nil)
	if err != nil {
		return 0, fmt.Errorf("failed to read offset: %w", err)
	}

	offset := binary.LittleEndian.Uint64(bz)
	return offset, nil
}

// TODO: implement.
func (s *LevelDBState) SaveFSM(interface{}) error {
	return nil
}

// TODO: implement.
func (s *LevelDBState) LoadFSM() (interface{}, error) {
	return nil, nil
}

func (s *LevelDBState) PutOperation(operation *Operation) error {
	s.Lock()
	defer s.Unlock()

	operations, err := s.getOperations()
	if err != nil {
		return fmt.Errorf("failed to getOperations: %w", err)
	}

	if _, ok := operations[operation.ID]; ok {
		return fmt.Errorf("operation %s already exists", operation.ID)
	}

	operations[operation.ID] = operation
	operationsJSON, err := json.Marshal(operations)
	if err != nil {
		return fmt.Errorf("failed to marshal operations: %w", err)
	}

	if err := s.stateDb.Put([]byte(operationsKey), operationsJSON, nil); err != nil {
		return fmt.Errorf("failed to put operations: %w", err)
	}

	return nil
}

func (s *LevelDBState) DeleteOperation(operationID string) error {
	s.Lock()
	defer s.Unlock()

	operations, err := s.getOperations()
	if err != nil {
		return fmt.Errorf("failed to getOperations: %w", err)
	}

	delete(operations, operationID)

	operationsJSON, err := json.Marshal(operations)
	if err != nil {
		return fmt.Errorf("failed to marshal operations: %w", err)
	}

	if err := s.stateDb.Put([]byte(operationsKey), operationsJSON, nil); err != nil {
		return fmt.Errorf("failed to put operations: %w", err)
	}

	return nil
}

func (s *LevelDBState) GetOperations() (map[string]*Operation, error) {
	s.Lock()
	defer s.Unlock()

	return s.getOperations()
}

func (s *LevelDBState) GetOperationByID(operationID string) (*Operation, error) {
	s.Lock()
	defer s.Unlock()

	operations, err := s.getOperations()
	if err != nil {
		return nil, fmt.Errorf("failed to getOperations: %w", err)
	}

	operation, ok := operations[operationID]
	if !ok {
		return nil, errors.New("operation not found")
	}

	return operation, nil
}

func (s *LevelDBState) getOperations() (map[string]*Operation, error) {
	bz, err := s.stateDb.Get([]byte(operationsKey), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get Operations (key: %s): %w", operationsKey, err)
	}

	var operations map[string]*Operation
	if err := json.Unmarshal(bz, &operations); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Operations: %w", err)
	}

	return operations, nil
}
