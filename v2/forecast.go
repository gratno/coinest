package main

import (
	"fmt"
	"github.com/shopspring/decimal"
	"strconv"
)

type depth [4]string

func (d *depth) CopyFrom(arr []string) {
	for i := 0; i < len(arr); i++ {
		d[i] = arr[i]
	}
}

func (d *depth) Clone() depth {
	p := depth{}
	for i := range *d {
		p[i] = (*d)[i]
	}
	return p
}

func (d *depth) Price() decimal.Decimal {
	p, err := decimal.NewFromString(d[0])
	if err != nil {
		panic("got unexpect price str:" + d[0])
	}
	return p
}

func (d *depth) Sheet() int {
	i, err := strconv.Atoi(d[1])
	if err != nil {
		panic("got unexpect sheet str:" + d[1])
	}
	return i
}

func (d *depth) Liquidation() int {
	i, err := strconv.Atoi(d[2])
	if err != nil {
		panic("got unexpect liquidation str:" + d[2])
	}
	return i
}

func (d *depth) Order() int {
	i, err := strconv.Atoi(d[3])
	if err != nil {
		panic("got unexpect order str:" + d[3])
	}
	return i
}

func (d *depth) Add(p depth) depth {
	d[1] = fmt.Sprintf("%d", d.Sheet()+p.Sheet())
	d[2] = fmt.Sprintf("%d", d.Liquidation()+p.Liquidation())
	d[3] = fmt.Sprintf("%d", d.Order()+p.Order())
	return *d
}

func (d *depth) Less(p depth) bool {
	return d.Sheet() > 500 && p.Sheet() > 2500 && 4*d.Sheet() < p.Sheet()
}

type futureModel struct {
}

func (m *futureModel) Future(markPrice decimal.Decimal, asks, bids []depth) (Trend, decimal.Decimal) {
	empty, many := asks[0], bids[0]
	emptyMerge, manyMerge := empty.Clone(), many.Clone()
	for i := 1; i < len(asks); i++ {
		emptyMerge = emptyMerge.Add(asks[i])
	}
	for i := 1; i < len(bids); i++ {
		manyMerge = manyMerge.Add(bids[i])
	}

	unknownPrice := empty.Price()
	if many.Sheet() > empty.Sheet() {
		unknownPrice = many.Price()
	}
	if emptyMerge.Less(manyMerge) && many.Sheet() > 2000 {
		if markPrice.GreaterThan(many.Price()) {
			return TREND_UNKNOWN, unknownPrice
		}
		return TREND_MANY, many.Price()
	}
	if manyMerge.Less(emptyMerge) && empty.Sheet() > 2000 {
		if markPrice.LessThan(empty.Price()) {
			return TREND_UNKNOWN, unknownPrice
		}
		return TREND_EMPTY, empty.Price()
	}
	return TREND_UNKNOWN, unknownPrice
}
