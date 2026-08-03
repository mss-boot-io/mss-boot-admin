/*
 * @Author: lwnmengjing
 * @Date: 2023/9/13 10:49:06
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2023/9/13 10:49:06
 */

package model

type PaginationImp interface {
	GetPage() int64
	GetPageSize() int64
}
