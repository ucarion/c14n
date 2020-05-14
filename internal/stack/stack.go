package stack

type Stack []map[string]entry

type entry struct {
	value string
	used  bool
}

func (s *Stack) Push(names map[string]string) {
	entries := make(map[string]entry, len(names))
	for k, v := range names {
		entries[k] = entry{value: v}
	}

	*s = append(*s, entries)
}

func (s *Stack) Pop() {
	*s = (*s)[:len(*s)-1]
}

func (s *Stack) Len() int {
	return len(*s)
}

func (s *Stack) Get(name string) string {
	for i := len(*s) - 1; i >= 0; i-- {
		if v, ok := (*s)[i][name]; ok {
			v.used = true
			(*s)[i][name] = v
			return v.value
		}
	}

	return ""
}

// func (s *Stack) Peek() map[string]string {
// 	if s.Len() == 0 {
// 		return nil
// 	}

// 	out := map[string]string{}
// 	for k, v := range (*s)[len(*s)-1] {
// 		out[k] = v.value
// 	}
// 	return out
// }

func (s *Stack) Used() map[string]string {
	out := map[string]string{}
	if len(*s) == 0 {
		return out
	}

	for k, v := range (*s)[len(*s)-1] {
		if v.used {
			out[k] = v.value
		}
	}

	return out
}

// func (s *Stack) IsEffective(name string) bool {
// 	// found := false
// 	val := ""
// 	for i := 0; i < len(*s)-1; i++ {
// 		if v, ok := (*s)[i][name]; ok {
// 			// found = true
// 			val = v.value
// 		}
// 	}

// 	// fmt.Println("effective compare", found, (*s)[len(*s)-1][name].value, val)
// 	// return !found || (*s)[len(*s)-1][name].value != val
// 	return (*s)[len(*s)-1][name].value != val
// }
