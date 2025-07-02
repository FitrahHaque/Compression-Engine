package flate

// func traceTree(node *huffman.CanonicalHuffmanNode, code uint32) {
// 	if node.IsLeaf {
// 		// code = huffman.Reverse(code, uint32(node.Item.GetLength()))
// 		// fmt.Printf("[ flate.traceTree ] symbol: %v ---- huffmanCode: %v, huffmanCodeLength: %v\n", node.Item.GetValue(), code, node.Item.GetLength())
// 		return
// 	}
// 	if node.Left != nil {
// 		traceTree(node.Left, code<<1)
// 	}
// 	if node.Right != nil {
// 		traceTree(node.Right, (code<<1)|1)
// 	}
// }
