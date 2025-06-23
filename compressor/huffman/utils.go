package huffman

import (
	"container/heap"
	"fmt"
	"slices"
	"sort"
)

type bitString string

type CanonicalHuffmanCode struct {
	Code   int
	Length int
}
type CanonicalHuffmanDecode struct {
	Symbol int
	Length int
}

type CanonicalHuffman interface {
	GetLength() int
	GetValue() int
}

type CanonicalHuffmanNode struct {
	Item        CanonicalHuffman
	IsLeaf      bool
	Left, Right *CanonicalHuffmanNode
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

func BuildCanonicalHuffmanEncoder(symbolFreq []int, lengthLimit int) ([]CanonicalHuffman, error) {
	symbolFreqMap := make(map[int32]int, len(symbolFreq))
	for symbol, freq := range symbolFreq {
		if freq > 0 {
			symbolFreqMap[int32(symbol)] = freq
		}
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
	if node, ok := root.(huffmanLeaf); ok {
		lengths[node.symbol] = 1
	} else {
		dfs(root, 0)
	}
	maxLength := 0
	for _, length := range lengths {
		maxLength = max(maxLength, length)
	}
	if maxLength > lengthLimit {
		return nil, fmt.Errorf("tree is longer than the limit %v\n", lengthLimit)
	}
	lengthCounts := make([]int, maxLength+1)
	var order []struct{ symbol, length int }
	for symbol, length := range lengths {
		if length == 0 {
			continue
		}
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
	fmt.Printf("[ BuildCanonicalHuffmanTree ] length: 0, count: %v\n", lengthCounts[0])
	for i := 1; i < len(lengthCounts); i++ {
		code = (code + lengthCounts[i-1]) << 1
		nextBaseCode[i] = code
		fmt.Printf("[ BuildCanonicalHuffmanTree ] length: %v, count: %v, nextBaseCode: %v\n", i, lengthCounts[i], nextBaseCode[i])
	}
	output := make([]CanonicalHuffman, len(symbolFreq))
	for _, info := range order {
		output[info.symbol] = CanonicalHuffmanCode{
			Code:   nextBaseCode[info.length],
			Length: info.length,
		}
		// checkMSBLength(output[info.symbol])
		nextBaseCode[info.length]++
	}
	return output, nil
}

func BuildCanonicalHuffmanDecoder(lengths []uint32) (*CanonicalHuffmanNode, error) {
	maxLength := uint32(0)
	for _, length := range lengths {
		maxLength = max(maxLength, length)
	}
	lengthCounts := make([]int, maxLength+1)
	var order []struct {
		symbol int
		length uint32
	}
	for symbol, length := range lengths {
		if length == 0 {
			continue
		}
		order = append(order, struct {
			symbol int
			length uint32
		}{
			symbol: symbol,
			length: length,
		})
		lengthCounts[length]++
	}
	sort.Slice(order, func(i, j int) bool {
		if order[i].length == order[j].length {
			return order[i].symbol < order[j].symbol
		}
		return order[i].length < order[j].length
	})
	nextBaseCode := make([]uint32, maxLength+1)
	code := 0
	for i := 1; i < len(lengthCounts); i++ {
		code = (code + lengthCounts[i-1]) << 1
		nextBaseCode[i] = uint32(code)
	}
	var root *CanonicalHuffmanNode
	for _, info := range order {
		item := CanonicalHuffmanDecode{
			Symbol: info.symbol,
			Length: int(info.length),
		}
		BuildCanonicalHuffmanTree(root, info.length, item, nextBaseCode[info.length])
		nextBaseCode[info.length]++
	}
	return root, nil
}

func (ch CanonicalHuffmanCode) GetLength() int {
	return ch.Length
}

func (ch CanonicalHuffmanCode) GetValue() int {
	return ch.Code
}

func (ch CanonicalHuffmanDecode) GetLength() int {
	return ch.Length
}

func (ch CanonicalHuffmanDecode) GetValue() int {
	return ch.Symbol
}

func BuildCanonicalHuffmanTree(node *CanonicalHuffmanNode, lengthRemaining uint32, item CanonicalHuffman, code uint32) {
	if lengthRemaining == 0 {
		node.Item = item
		node.IsLeaf = true
		return
	}
	bit := code & 1
	code >>= 1
	lengthRemaining--
	if bit == 0 {
		if node.Left == nil {
			node.Left = &CanonicalHuffmanNode{}
		}
		BuildCanonicalHuffmanTree(node.Left, lengthRemaining, item, code)
	} else {
		if node.Right == nil {
			node.Right = &CanonicalHuffmanNode{}
		}
		BuildCanonicalHuffmanTree(node.Right, lengthRemaining, item, code)
	}
}
