package player

import (
	"math/rand"

	"github.com/Flipez/subtonic/api"
)

type RepeatMode int

const (
	RepeatOff RepeatMode = iota
	RepeatAll
	RepeatOne
)

type Queue struct {
	songs   []api.Song
	index   int
	shuffle bool
	order   []int // shuffled index mapping; nil when shuffle off
	repeat  RepeatMode
}

func (q *Queue) Set(songs []api.Song, start int) {
	q.songs = songs
	q.index = start
	q.order = nil
	if q.shuffle && len(songs) > 0 {
		q.buildShuffleOrder()
	}
}

func (q *Queue) Current() (api.Song, bool) {
	if len(q.songs) == 0 || q.index < 0 || q.index >= len(q.songs) {
		return api.Song{}, false
	}
	return q.songs[q.actualIndex(q.index)], true
}

func (q *Queue) Next() (api.Song, bool) {
	if len(q.songs) == 0 {
		return api.Song{}, false
	}
	if q.repeat == RepeatOne {
		return q.songs[q.actualIndex(q.index)], true
	}
	if q.index+1 >= len(q.songs) {
		if q.repeat == RepeatAll {
			q.index = 0
			return q.songs[q.actualIndex(q.index)], true
		}
		return api.Song{}, false
	}
	q.index++
	return q.songs[q.actualIndex(q.index)], true
}

func (q *Queue) Prev() (api.Song, bool) {
	if len(q.songs) == 0 {
		return api.Song{}, false
	}
	if q.repeat == RepeatOne {
		return q.songs[q.actualIndex(q.index)], true
	}
	if q.index <= 0 {
		if q.repeat == RepeatAll {
			q.index = len(q.songs) - 1
			return q.songs[q.actualIndex(q.index)], true
		}
		return api.Song{}, false
	}
	q.index--
	return q.songs[q.actualIndex(q.index)], true
}

func (q *Queue) Add(song api.Song) {
	q.songs = append(q.songs, song)
	if q.shuffle && q.order != nil {
		q.order = append(q.order, len(q.songs)-1)
	}
}

// InsertNext adds a song right after the currently playing position.
func (q *Queue) InsertNext(song api.Song) {
	if len(q.songs) == 0 {
		q.songs = append(q.songs, song)
		return
	}
	cur := q.actualIndex(q.index)
	insertAt := cur + 1
	// Insert into the songs slice
	q.songs = append(q.songs, api.Song{})
	copy(q.songs[insertAt+1:], q.songs[insertAt:])
	q.songs[insertAt] = song

	// Update shuffle order if active
	if q.order != nil {
		// Bump all order entries >= insertAt
		for i, v := range q.order {
			if v >= insertAt {
				q.order[i] = v + 1
			}
		}
		// Insert the new song right after current logical position
		newOrder := make([]int, 0, len(q.order)+1)
		newOrder = append(newOrder, q.order[:q.index+1]...)
		newOrder = append(newOrder, insertAt)
		newOrder = append(newOrder, q.order[q.index+1:]...)
		q.order = newOrder
	}
}

func (q *Queue) Songs() []api.Song {
	return q.songs
}

func (q *Queue) Index() int {
	return q.actualIndex(q.index)
}

// actualIndex maps the logical queue position to the real songs slice index.
func (q *Queue) actualIndex(i int) int {
	if q.order != nil && i >= 0 && i < len(q.order) {
		return q.order[i]
	}
	return i
}

func (q *Queue) ToggleShuffle() {
	if q.shuffle {
		// Turning off: find current song's position in original order
		if q.order != nil && q.index >= 0 && q.index < len(q.order) {
			q.index = q.order[q.index]
		}
		q.order = nil
		q.shuffle = false
	} else {
		q.shuffle = true
		if len(q.songs) > 0 {
			q.buildShuffleOrder()
		}
	}
}

func (q *Queue) buildShuffleOrder() {
	n := len(q.songs)
	q.order = make([]int, n)
	for i := range q.order {
		q.order[i] = i
	}
	// Fisher-Yates shuffle
	for i := n - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		q.order[i], q.order[j] = q.order[j], q.order[i]
	}
	// Place the current song at position 0 so playback doesn't restart
	currentActual := q.index
	for i, v := range q.order {
		if v == currentActual {
			q.order[0], q.order[i] = q.order[i], q.order[0]
			break
		}
	}
	q.index = 0
}

func (q *Queue) CycleRepeat() {
	switch q.repeat {
	case RepeatOff:
		q.repeat = RepeatAll
	case RepeatAll:
		q.repeat = RepeatOne
	case RepeatOne:
		q.repeat = RepeatOff
	}
}

func (q *Queue) Shuffle() bool {
	return q.shuffle
}

func (q *Queue) Repeat() RepeatMode {
	return q.repeat
}

func (q *Queue) Remove(idx int) {
	if idx < 0 || idx >= len(q.songs) {
		return
	}
	q.songs = append(q.songs[:idx], q.songs[idx+1:]...)

	// Update shuffle order if active
	if q.order != nil {
		// Remove the entry that points to idx, and adjust entries > idx
		newOrder := make([]int, 0, len(q.order)-1)
		removedPos := -1
		for i, v := range q.order {
			if v == idx {
				removedPos = i
				continue
			}
			if v > idx {
				v--
			}
			newOrder = append(newOrder, v)
		}
		q.order = newOrder
		// Adjust logical index
		if removedPos >= 0 && removedPos < q.index {
			q.index--
		} else if removedPos == q.index {
			if q.index >= len(q.order) {
				q.index = len(q.order) - 1
			}
		}
	} else {
		if idx < q.index {
			q.index--
		} else if idx == q.index {
			if q.index >= len(q.songs) {
				q.index = len(q.songs) - 1
			}
		}
	}
	if q.index < 0 {
		q.index = 0
	}
}

func (q *Queue) Move(from, to int) {
	if from < 0 || from >= len(q.songs) || to < 0 || to >= len(q.songs) || from == to {
		return
	}
	song := q.songs[from]
	// Remove from old position
	q.songs = append(q.songs[:from], q.songs[from+1:]...)
	// Insert at new position
	q.songs = append(q.songs[:to], append([]api.Song{song}, q.songs[to:]...)...)

	// Adjust current index (actual index, not logical)
	cur := q.actualIndex(q.index)
	if cur == from {
		cur = to
	} else {
		if from < cur {
			cur--
		}
		if to <= cur {
			cur++
		}
	}

	// Rebuild shuffle order if active
	if q.order != nil {
		for i, v := range q.order {
			if v == from {
				q.order[i] = to
			} else {
				adjusted := v
				if v > from {
					adjusted--
				}
				if adjusted >= to {
					adjusted++
				}
				q.order[i] = adjusted
			}
		}
	} else {
		q.index = cur
	}
}

func (q *Queue) Clear() {
	q.songs = nil
	q.index = 0
	q.order = nil
}

// SetIndex sets the current queue index (actual songs index) for direct playback.
func (q *Queue) SetIndex(idx int) {
	if q.order != nil {
		// Find the logical position for this actual index
		for i, v := range q.order {
			if v == idx {
				q.index = i
				return
			}
		}
	}
	q.index = idx
}
