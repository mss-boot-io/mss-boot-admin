package pkg

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/1/28 11:09:48
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/1/28 11:09:48
 */

type TreeImp interface {
	GetIndex() string
	GetParentID() string
	AddChildren([]TreeImp)
	SortChildren()
}

// BuildTree 使用递归实现树
func BuildTree(list []TreeImp, parentID string) []TreeImp {
	if len(list) == 0 {
		return nil
	}
	var tree []TreeImp
	for i := range list {
		if list[i].GetParentID() == parentID {
			list[i].AddChildren(BuildTree(list, list[i].GetIndex()))
			list[i].SortChildren()
			tree = append(tree, list[i])
		}
	}
	return tree
}
