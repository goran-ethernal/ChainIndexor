package store

import (
	"github.com/ethereum/go-ethereum/common"
)

type UnsyncedTopics struct {
	addrToTopicCoverage map[common.Address]map[common.Hash]CoverageRange
}

func NewUnsyncedTopics() *UnsyncedTopics {
	return &UnsyncedTopics{
		addrToTopicCoverage: make(map[common.Address]map[common.Hash]CoverageRange),
	}
}

func (ut *UnsyncedTopics) IsEmpty() bool {
	return len(ut.addrToTopicCoverage) == 0
}

func (ut *UnsyncedTopics) ShouldCatchUp(lastIndexedBlock, downloaderStartBlock uint64) bool {
	if lastIndexedBlock <= downloaderStartBlock {
		return false
	}

	for _, topicMap := range ut.addrToTopicCoverage {
		for _, coverage := range topicMap {
			if coverage.ToBlock < lastIndexedBlock {
				return true
			}
		}
	}
	return false
}

func (ut *UnsyncedTopics) ContainsAddress(address common.Address) bool {
	_, exists := ut.addrToTopicCoverage[address]
	return exists
}

func (ut *UnsyncedTopics) ContainsTopic(address common.Address, topic common.Hash) bool {
	topics, exists := ut.addrToTopicCoverage[address]
	if !exists {
		return false
	}
	_, topicExists := topics[topic]
	return topicExists
}

func (ut *UnsyncedTopics) AddTopic(address common.Address, topic common.Hash, coverage CoverageRange) {
	if _, exists := ut.addrToTopicCoverage[address]; !exists {
		ut.addrToTopicCoverage[address] = make(map[common.Hash]CoverageRange)
	}

	ut.addrToTopicCoverage[address][topic] = coverage
}

func (ut *UnsyncedTopics) GetAddressesAndTopics() ([]common.Address, [][]common.Hash, uint64) {
	addresses := make([]common.Address, 0, len(ut.addrToTopicCoverage))
	topics := make([][]common.Hash, 0, len(ut.addrToTopicCoverage))
	minCoveredBlock := ^uint64(0) // Max uint64

	for addr, topicMap := range ut.addrToTopicCoverage {
		addresses = append(addresses, addr)

		topicList := make([]common.Hash, 0, len(topicMap))
		for topic, coverage := range topicMap {
			topicList = append(topicList, topic)
			if coverage.ToBlock < minCoveredBlock {
				minCoveredBlock = coverage.ToBlock
			}
		}

		topics = append(topics, topicList)
	}

	return addresses, topics, minCoveredBlock
}
