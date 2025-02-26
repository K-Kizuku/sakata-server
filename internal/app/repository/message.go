package repository

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/dgraph-io/ristretto"
)

type MessageRepository struct {
	cache     *ristretto.Cache
	sortedSet *SortedSet
	mu        sync.Mutex
}

type IMessageRepository interface {
	AddMessage(ctx context.Context, timestamp int64, message string) error
	GetMessagesByTimeRange(ctx context.Context, currentTimestamp, timeRange int64) (map[string]int, error)
	GetMessagesFromMock(ctx context.Context, currentTimestamp, timeRange int64) (map[string]int, error)
}

func NewMessageRepository(cache *ristretto.Cache) IMessageRepository {
	return &MessageRepository{
		cache:     cache,
		sortedSet: NewSortedSet(),
	}
}

type sortedSetEntry struct {
	key   string
	score int64
}

type SortedSet struct {
	entries []sortedSetEntry
}

func NewSortedSet() *SortedSet {
	return &SortedSet{
		entries: make([]sortedSetEntry, 0),
	}
}

func (s *SortedSet) Add(key string, score int64) {
	for i, entry := range s.entries {
		if entry.key == key {
			s.entries[i].score = score
			s.resort()
			return
		}
	}
	s.entries = append(s.entries, sortedSetEntry{key: key, score: score})
	s.resort()
}

func (s *SortedSet) resort() {
	sort.Slice(s.entries, func(i, j int) bool {
		return s.entries[i].score < s.entries[j].score
	})
}

func (s *SortedSet) RangeByScore(min, max int64) []string {
	var result []string
	for _, entry := range s.entries {
		if entry.score >= min && entry.score <= max {
			result = append(result, entry.key)
		}
	}
	return result
}

func (mr *MessageRepository) AddMessage(ctx context.Context, timestamp int64, message string) error {
	hashKey := fmt.Sprintf("timestamp:%d", timestamp)

	mr.mu.Lock()
	defer mr.mu.Unlock()

	var hash map[string]int
	if val, found := mr.cache.Get(hashKey); found {
		hash = val.(map[string]int)
	} else {
		hash = make(map[string]int)
	}

	hash[message]++

	mr.cache.SetWithTTL(hashKey, hash, 1, time.Minute)

	mr.sortedSet.Add(hashKey, timestamp)

	return nil
}

func (mr *MessageRepository) GetMessagesByTimeRange(ctx context.Context, currentTimestamp, timeRange int64) (map[string]int, error) {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	minScore := currentTimestamp - timeRange
	maxScore := currentTimestamp
	keys := mr.sortedSet.RangeByScore(minScore, maxScore)

	aggregatedData := make(map[string]int)
	for _, hashKey := range keys {
		if val, found := mr.cache.Get(hashKey); found {
			hash := val.(map[string]int)
			for msg, count := range hash {
				aggregatedData[msg] += count
			}
		}
	}
	return aggregatedData, nil
}

func (mr *MessageRepository) GetMessagesFromMock(ctx context.Context, currentTimestamp, timeRange int64) (map[string]int, error) {
	aggregatedData := make(map[string]int)
	for i := 0; i < 10; i++ {
		ii := i * 10
		aggregatedData["message"+strconv.Itoa(ii)] = ii
	}
	return aggregatedData, nil
}
