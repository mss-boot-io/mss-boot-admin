package mongodb

/*
 * @Author: lwnmengjing
 * @Date: 2022/3/14 9:32
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2022/3/14 9:32
 */

import mgm "github.com/kamva/mgm/v3"

// Tabler table interface
type Tabler interface {
	mgm.Model
	Make()
}
