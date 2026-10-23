package constant

type MultiKeyMode string

const (
	MultiKeyModeRandom  MultiKeyMode = "random"  // 随机
	MultiKeyModePolling MultiKeyMode = "polling" // 轮询
	MultiKeyModeDrain   MultiKeyMode = "drain"   // 耗尽切换：固定使用当前 key，直到它被禁用后才顺延到下一个
)
