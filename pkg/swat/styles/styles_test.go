package styles_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sergeii/swat4master/pkg/swat/styles"
)

func TestClean(t *testing.T) {
	tests := []struct {
		styled string
		want   string
	}{
		{`  Serge  `, `Serge`},
		{`[i]Serge[\i]`, `[i]Serge[\i]`},
		{`[c=FF0000]Serge`, `Serge`},
		{`[c=F]Serge`, `Serge`},
		{`[c=]Serge`, `Serge`},
		{`[c]Serge[\c]`, `Serge`},
		{`[C]Serge[\C]`, `Serge`},
		{`[C]Serge[\c]`, `Serge`},
		{`[c]Serge[\C]`, `Serge`},
		{` [c=FF0000]Serge`, `Serge`},
		{`[c=FF0001][u]Serge[b]`, `Serge`},
		{`[c=FF[u]003[\u]0][u]Serge[b][c=FF00]`, `Serge`},
		{`[c=FFFF00]`, ``},
		{`[C=FFFF00]`, ``},
		{`[b][u][\u]`, ``},
		{`[b][U][\U]`, ``},
		{`[b] [u]  [\u] `, ``},
		{`[c=704070][b]M[c=A080A0]a[c=D0C0D0]i[c=FFFFFF]n`, `Main`},
		{`[c=F4F4F4][b]Kee[c=E9E9E9]p u[c=DEDEDE]r h[c=D3D3D3]ead[c=C8C8C8] do[c=BDBDBD]wn`, `Keep ur head down`},
	}
	for _, tt := range tests {
		t.Run(tt.styled, func(t *testing.T) {
			got := styles.Clean(tt.styled)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestToHTML(t *testing.T) {
	tests := []struct {
		styled string
		want   string
	}{
		{`[c=FF00FF]`, ``},
		{`[c=FF00FF][\c]`, ``},
		{`[c=FF00FF][c=FFFFFF]`, ``},
		{
			`[c=FF00FF]Swat4 Server`,
			`<span style="color:#FF00FF;">Swat4 Server</span>`,
		},
		{
			`[c=FF00FF]Swat4 Server[\c]`,
			`<span style="color:#FF00FF;">Swat4 Server</span>`,
		},
		{
			`[C=FF00FF]Swat4 Server[\c]`,
			`<span style="color:#FF00FF;">Swat4 Server</span>`,
		},
		{
			`[C=FF00FF]Swat4 Server[\C]`,
			`<span style="color:#FF00FF;">Swat4 Server</span>`,
		},
		{
			`[c=FF00FF]Swat4[\c] Server`,
			`<span style="color:#FF00FF;">Swat4</span> Server`,
		},
		{
			`[c=FF00FF][b][u]Swat4[\u][C=0000FF]Server[\c]`,
			`<span style="color:#FF00FF;">Swat4</span><span style="color:#0000FF;">Server</span>`,
		},
		{
			`[b][u][c=FF00FF]Swat4[C=0000FF]Server[\c][\u]`,
			`<span style="color:#FF00FF;">Swat4</span><span style="color:#0000FF;">Server</span>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.styled, func(t *testing.T) {
			got := styles.ToHTML(tt.styled)
			assert.Equal(t, tt.want, got)
		})
	}
}
