package huffman

import (
	"container/heap"
	"slices"
)

type bitString string

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
