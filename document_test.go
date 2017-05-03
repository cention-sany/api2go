package api2go_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	. "github.com/cention-sany/api2go"
)

const (
	numStr  = "number"
	sizeStr = "size"
	offStr  = "offset"
	lmtStr  = "limit"
)

var tstPgTbl1 = []struct {
	data map[string]string
	has  bool
	o, l int
	err  error
}{
	{data: map[string]string{}, has: false, o: 0, l: 0, err: nil},
	{data: nil, has: false, o: 0, l: 0, err: nil},
	{ // number is ignored as size is not present
		data: map[string]string{numStr: "10", lmtStr: "10"},
		has:  true, o: 0, l: 10, err: nil,
	},
	{ // size is presented as non-convertable to digit
		data: map[string]string{numStr: "1", sizeStr: ""},
		has:  false, o: 0, l: 0, err: errors.New(`strconv.Atoi: parsing "": invalid syntax`),
	},
	{ // size can not be zero
		data: map[string]string{numStr: "1", sizeStr: "0"},
		has:  false, o: 0, l: 0, err: errors.New("api2go: invalid page size"),
	},
	{ // size can not less than 0
		data: map[string]string{numStr: "1", sizeStr: "-1"},
		has:  false, o: 0, l: 0, err: errors.New("api2go: invalid page size"),
	},
	{ // number can not less than 0
		data: map[string]string{numStr: "0", sizeStr: "10"},
		has:  false, o: 0, l: 0, err: errors.New("api2go: invalid page number"),
	},
	{ // number must be integer
		data: map[string]string{numStr: "NotInt", sizeStr: "10"},
		has:  false, o: 0, l: 0, err: errors.New(`strconv.Atoi: parsing "NotInt": invalid syntax`),
	},
	{
		data: map[string]string{sizeStr: "10"},
		has:  true, o: 0, l: 10, err: nil,
	},
	{
		data: map[string]string{numStr: "1", sizeStr: "5"},
		has:  true, o: 0, l: 5, err: nil,
	},
	{
		data: map[string]string{numStr: "2", sizeStr: "6"},
		has:  true, o: 6, l: 6, err: nil,
	},
	{
		data: map[string]string{numStr: "4", sizeStr: "7"},
		has:  true, o: 21, l: 7, err: nil,
	},
	{
		data: map[string]string{offStr: "1", lmtStr: "0"},
		has:  false, o: 0, l: 0, err: errors.New("api2go: invalid page limit"),
	},
	{
		data: map[string]string{offStr: "2", lmtStr: "-3"},
		has:  false, o: 0, l: 0, err: errors.New("api2go: invalid page limit"),
	},
	{
		data: map[string]string{offStr: "-2", lmtStr: "NotInt"},
		has:  false, o: 0, l: 0, err: errors.New(`strconv.Atoi: parsing "NotInt": invalid syntax`),
	},
	{
		data: map[string]string{offStr: "NotInt"},
		has:  false, o: 0, l: 0, err: errors.New(`strconv.Atoi: parsing "NotInt": invalid syntax`),
	},
	{
		data: map[string]string{offStr: "1"},
		has:  true, o: 1, l: -1, err: nil,
	},
	{
		data: map[string]string{lmtStr: "10"},
		has:  true, o: 0, l: 10, err: nil,
	},
	{
		data: map[string]string{"other[filter]": "10"},
		has:  false, o: 0, l: 0, err: nil,
	},
}

func TestOffsetPage(t *testing.T) {
	r := new(Request)
	for n, _ := range tstPgTbl1 {
		d := &tstPgTbl1[n]
		n++
		r.Pagination = d.data
		has, offset, limit, err := OffsetPage(r)
		if has != d.has {
			var expStr, resStr string
			if d.has {
				expStr = "has"
				resStr = "no"
			} else {
				expStr = "no"
				resStr = "has"
			}
			t.Errorf("#%d: Expect %s query but %s query.", n, expStr, resStr)
		} else if offset != d.o {
			t.Errorf("#%d: Expect offset %d but got %d.", n, d.o, offset)
		} else if limit != d.l {
			t.Errorf("#%d: Expect offset %d but got %d.", n, d.l, offset)
		} else if d.err == nil && err != nil {
			t.Error("#", strconv.Itoa(n), ": Expect no error but got:", err)
		} else if d.err != nil {
			if err == nil {
				t.Error("#", strconv.Itoa(n), ": Expect error ", d.err,
					" but got no error.")
			} else if !strings.Contains(err.Error(), d.err.Error()) {
				t.Error("#", strconv.Itoa(n), ": Expect error ", d.err,
					" but got error:", err)
			}
		}
	}
}
