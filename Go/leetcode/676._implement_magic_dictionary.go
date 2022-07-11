type MagicDictionary struct {
    hashMap map[int]map[string]bool
}

func Constructor() MagicDictionary {
    return MagicDictionary{hashMap: make(map[int]map[string]bool)}
}

func (this *MagicDictionary) BuildDict(dictionary []string)  {
    for _, v := range dictionary {
        if this.hashMap[len(v)] == nil {
            this.hashMap[len(v)] = make(map[string]bool)
        }
        this.hashMap[len(v)][v] = true
    }
}

func (this *MagicDictionary) Search(searchWord string) bool {
    length := len(searchWord)
    t := this.hashMap[length]
    for key := range t {
        if key != searchWord && isMatched(key, searchWord) {
            return true
        }
    }
    return false
}

func isMatched(s1, s2 string) bool {
    var flag bool
    for i := 0; i < len(s1); i++ {
        if s1[i] != s2[i] {
            if flag {
                return false
            }
            flag = true
        }
    }
    return true
}
