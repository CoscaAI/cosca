package policy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToolBudgetConsumeThenExhaust(t *testing.T) {
	b := NewToolBudget(2)
	assert.Equal(t, 2, b.Max)

	assert.True(t, b.Consume(), "primeira chamada deve passar")
	assert.True(t, b.Consume(), "segunda chamada deve passar")
	assert.False(t, b.Consume(), "terceira chamada deve estourar")
	assert.Equal(t, 2, b.Calls)
	assert.False(t, b.Consume(), "contínua estourado")
	assert.Equal(t, 2, b.Calls, "não incrementa após estourar")
}

func TestToolBudgetReset(t *testing.T) {
	b := NewToolBudget(2)
	b.Consume()
	b.Consume()
	assert.False(t, b.Consume())

	b.Reset()
	assert.Equal(t, 0, b.Calls)
	assert.True(t, b.Consume(), "após reset volta a permitir")
	assert.True(t, b.Consume())
	assert.False(t, b.Consume())
}

func TestToolBudgetDefaultMax(t *testing.T) {
	// NewToolBudget(0) normaliza para o default da casa.
	b := NewToolBudget(0)
	assert.Equal(t, DefaultMaxToolCalls, b.Max)

	// Construção manual com Max<=0 também usa o default no Consume.
	b2 := &ToolBudget{Max: 0}
	for i := 0; i < DefaultMaxToolCalls; i++ {
		assert.True(t, b2.Consume(), "chamada %d deveria passar", i)
	}
	assert.False(t, b2.Consume(), "default deve ser exatamente %d", DefaultMaxToolCalls)
}

func TestToolBudgetNil(t *testing.T) {
	var b *ToolBudget
	assert.False(t, b.Consume(), "nil budget nunca autoriza")
	b.Reset() // não deve panic
}
