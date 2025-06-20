package huffman

import (
	"container/heap"
	"errors"
	"slices"
	"sort"
)

type bitString string

type CanonicalHuffmanCode struct {
	Code   int
	Length int
}
type huffmanTree interface {
	getFrequency() int
	getId() int
}
type huffmanLeaf struct {
	freq, id int
	symbol   rune
}
type huffmanNode struct {
	freq, id    int
	left, right huffmanTree
}

type huffmanHeap []huffmanTree

func (hub *huffmanHeap) Push(item any) {
	*hub = append(*hub, item.(huffmanTree))
}

func (hub *huffmanHeap) Pop() any {
	popped := (*hub)[len(*hub)-1]
	(*hub) = (*hub)[:len(*hub)-1]
	return popped
}

func (hub huffmanHeap) Len() int {
	return len(hub)
}

func (hub huffmanHeap) Less(i, j int) bool {
	if hub[i].getFrequency() != hub[j].getFrequency() {
		return hub[i].getFrequency() < hub[j].getFrequency()
	}
	return hub[i].getId() < hub[j].getId()
}

func (hub huffmanHeap) Swap(i, j int) {
	hub[i], hub[j] = hub[j], hub[i]
}

func (leaf huffmanLeaf) getId() int {
	return leaf.id
}

func (leaf huffmanLeaf) getFrequency() int {
	return leaf.freq
}

func (node huffmanNode) getFrequency() int {
	return node.freq
}

func (node huffmanNode) getId() int {
	return node.id
}

func buildTree(symbolFreq map[rune]int) huffmanTree {
	var keys []rune
	for r := range symbolFreq {
		keys = append(keys, r)
	}
	slices.Sort(keys)
	var treehub huffmanHeap
	monoId := 0
	for _, key := range keys {
		treehub = append(treehub, huffmanLeaf{
			freq:   symbolFreq[key],
			symbol: key,
			id:     monoId,
		})
		monoId++
	}
	// for _, t := range treehub {
	// 	p := t.(huffmanLeaf)
	// 	fmt.Printf("[ buildTree ] symbol: %v --- freq: %v --- id: %v\n", string(p.symbol), p.freq, p.id)
	// }
	heap.Init(&treehub)
	for treehub.Len() > 1 {
		x := heap.Pop(&treehub).(huffmanTree)
		y := heap.Pop(&treehub).(huffmanTree)
		heap.Push(&treehub, huffmanNode{
			freq:  x.getFrequency() + y.getFrequency(),
			left:  x,
			right: y,
			id:    monoId,
		})
		monoId++
	}
	return heap.Pop(&treehub).(huffmanTree)
}

func BuildCanonicalHuffmanTree(symbolFreq []int, lengthLimit int) ([]CanonicalHuffmanCode, error) {
	symbolFreqMap := make(map[int32]int, len(symbolFreq))
	for symbol, freq := range symbolFreq {
		symbolFreqMap[int32(symbol)] = freq
	}
	lengths := make([]int, len(symbolFreq))
	root := buildTree(symbolFreqMap)
	var dfs func(huffmanTree, int)
	dfs = func(tree huffmanTree, len int) {
		switch node := tree.(type) {
		case huffmanLeaf:
			lengths[node.symbol] = len
			return
		case huffmanNode:
			dfs(node.left, len+1)
			dfs(node.right, len+1)
			return
		}
	}
	dfs(root, 0)
	maxLength := 0
	for _, length := range lengths {
		maxLength = max(maxLength, length)
	}
	if maxLength > lengthLimit {
		return nil, errors.New("tree is longer than limit")
	}
	lengthCounts := make([]int, maxLength+1)
	var order []struct{ symbol, length int }
	for symbol, length := range lengths {
		order = append(order, struct {
			symbol int
			length int
		}{symbol: symbol, length: length})
		lengthCounts[length]++
	}
	sort.Slice(order, func(i, j int) bool {
		if order[i].length == order[j].length {
			return order[i].symbol < order[j].symbol
		}
		return order[i].length < order[j].length
	})
	nextBaseCode := make([]int, maxLength+1)
	code := 0
	for i := 1; i < len(lengthCounts); i++ {
		code = (code + lengthCounts[i-1]) << 1
		nextBaseCode[i] = code
	}
	output := make([]CanonicalHuffmanCode, len(symbolFreq))
	for _, info := range order {
		if info.length == 0 {
			continue
		}
		output[info.symbol] = CanonicalHuffmanCode{
			Code:   nextBaseCode[info.length],
			Length: info.length,
		}
		nextBaseCode[info.length]++
	}
	return output, nil
}
