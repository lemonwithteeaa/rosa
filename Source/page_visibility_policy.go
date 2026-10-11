package main

// pageControlMayBeVisible is the page ownership rule at the native HWND boundary.
// During startup active is zero. A page being prepared also stays invisible
// until its content and geometry have both been committed.
func pageControlMayBeVisible(owner, active int, requested, preparing bool) bool {
	return requested && owner != 0 && owner == active && !preparing
}
