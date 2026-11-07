package migemo

// generatorNode is a node of a ternary tree: the siblings at one position of a
// word form an AA tree, and child holds the next position.
type generatorNode struct {
	value       uint16
	child       *generatorNode
	left, right *generatorNode
	level       int
}

func skew(t *generatorNode) *generatorNode {
	if t == nil || t.left == nil || t.left.level != t.level {
		return t
	}
	l := t.left
	t.left = l.right
	l.right = t
	return l
}

func split(t *generatorNode) *generatorNode {
	if t == nil || t.right == nil || t.right.right == nil || t.level != t.right.right.level {
		return t
	}
	r := t.right
	t.right = r.left
	r.left = t
	r.level++
	return r
}

// insert returns the new sibling root and the node holding x.
func insert(x uint16, t *generatorNode) (root, target *generatorNode, inserted bool) {
	switch {
	case t == nil:
		r := &generatorNode{value: x, level: 1}
		return r, r, true
	case x < t.value:
		t.left, target, inserted = insert(x, t.left)
	case x > t.value:
		t.right, target, inserted = insert(x, t.right)
	default:
		return t, t, false
	}
	return split(skew(t)), target, inserted
}

// addWord keeps only the shortest of two words where one starts with the
// other, since the pattern of the shorter one matches the longer one too.
func addWord(node *generatorNode, word []uint16) *generatorNode {
	if len(word) == 0 {
		return nil
	}
	root, target, inserted := insert(word[0], node)
	if inserted || target.child != nil {
		target.child = addWord(target.child, word[1:])
	}
	return root
}

func (n *generatorNode) siblings(visit func(*generatorNode)) {
	if n != nil {
		n.left.siblings(visit)
		visit(n)
		n.right.siblings(visit)
	}
}

// generator builds one regular expression out of the words added to it.
type generator struct {
	or, startGroup, endGroup, startClass, endClass, newline []uint16
	escape                                                  [128]bool
	root                                                    *generatorNode
}

func newGenerator(r Rxop) *generator {
	g := &generator{
		or:         toUnits(r.Or),
		startGroup: toUnits(r.StartGroup),
		endGroup:   toUnits(r.EndGroup),
		startClass: toUnits(r.StartClass),
		endClass:   toUnits(r.EndClass),
		newline:    toUnits(r.Newline),
	}
	for _, c := range r.Escape {
		g.escape[c] = true
	}
	return g
}

func (g *generator) add(word []uint16) {
	if len(word) > 0 {
		g.root = addWord(g.root, word)
	}
}

func (g *generator) appendChar(buf []uint16, c uint16) []uint16 {
	if c < 128 && g.escape[c] {
		buf = append(buf, '\\')
	}
	return append(buf, c)
}

func (g *generator) appendNode(buf []uint16, node *generatorNode) []uint16 {
	brother, haschild := 0, 0
	node.siblings(func(n *generatorNode) {
		brother++
		if n.child != nil {
			haschild++
		}
	})
	nochild := brother - haschild
	grouped := brother > 1 && haschild > 0
	if grouped {
		buf = append(buf, g.startGroup...)
	}
	if nochild > 0 {
		if nochild > 1 {
			buf = append(buf, g.startClass...)
		}
		node.siblings(func(n *generatorNode) {
			if n.child == nil {
				buf = g.appendChar(buf, n.value)
			}
		})
		if nochild > 1 {
			buf = append(buf, g.endClass...)
		}
	}
	if haschild > 0 {
		if nochild > 0 {
			buf = append(buf, g.or...)
		}
		first := true
		node.siblings(func(n *generatorNode) {
			if n.child == nil {
				return
			}
			if !first {
				buf = append(buf, g.or...)
			}
			first = false
			buf = g.appendChar(buf, n.value)
			buf = append(buf, g.newline...)
			buf = g.appendNode(buf, n.child)
		})
	}
	if grouped {
		buf = append(buf, g.endGroup...)
	}
	return buf
}

func (g *generator) generate() []uint16 {
	if g.root == nil {
		return nil
	}
	return g.appendNode(nil, g.root)
}
