package integrity

// addToHistory adds a result to the check history
func (c *Checker) addToHistory(result CheckResult) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.checkHistory = append(c.checkHistory, result)
	if len(c.checkHistory) > c.maxHistory {
		c.checkHistory = c.checkHistory[1:]
	}
}

// GetHistory returns recent check results
func (c *Checker) GetHistory(limit int) []CheckResult {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if limit <= 0 || limit > len(c.checkHistory) {
		limit = len(c.checkHistory)
	}

	start := len(c.checkHistory) - limit
	result := make([]CheckResult, limit)
	copy(result, c.checkHistory[start:])
	return result
}
