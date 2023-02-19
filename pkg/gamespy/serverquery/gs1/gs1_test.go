package gs1_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/sergeii/swat4master/pkg/gamespy/serverquery/gs1"
)

type b []byte

func TestQuery_QueryIDIsNotZeroBased(t *testing.T) {
	responses := make(chan []byte)
	go func() {
		responses <- b("\\hostname\\test\\queryid\\1")
		responses <- b("\\hostport\\10480\\queryid\\2\\final\\")
	}()
	server, cancel := gs1.PrepareGS1Server(responses)
	defer cancel()
	resp, err := gs1.Query(context.TODO(), server.LocalAddrPort(), time.Millisecond*10)
	assert.NoError(t, err)
	assert.Contains(t, resp.Fields, "final")
	assert.Equal(t, "", resp.Fields["final"])
	assert.Equal(t, "10480", resp.Fields["hostport"])
	assert.Equal(t, "test", resp.Fields["hostname"])
}

func TestQuery_VanillaProtocolIsSupported(t *testing.T) {
	responses := make(chan []byte)
	go func() {
		responses <- b(
			"\\hostname\\[C=FFFF00]WWW.HOUSEOFPAiN.TK (Antics)\\numplayers\\4" +
				"\\maxplayers\\12\\gametype\\Barricaded Suspects\\gamevariant\\SWAT 4" +
				"\\mapname\\The Wolcott Projects\\hostport\\10480\\password\\0\\gamever\\1.0" +
				"\\player_0\\Navis\\player_1\\TAMAL(SPEC)\\player_2\\Player\\player_3\\Osanda(VIEW)" +
				"\\score_0\\15\\score_1\\0\\score_2\\3\\score_3\\0\\ping_0\\56\\ping_1\\160" +
				"\\ping_2\\256\\ping_3\\262\\final\\\\queryid\\1.1",
		)
	}()
	server, cancel := gs1.PrepareGS1Server(responses)
	defer cancel()
	resp, err := gs1.Query(context.TODO(), server.LocalAddrPort(), time.Millisecond*10)
	assert.NoError(t, err)
	assert.Equal(t, "[C=FFFF00]WWW.HOUSEOFPAiN.TK (Antics)", resp.Fields["hostname"])
	assert.Equal(t, "4", resp.Fields["numplayers"])
	assert.Equal(t, "1.1", resp.Fields["queryid"])
	assert.Len(t, resp.Players, 4)
	assert.Equal(t, "Navis", resp.Players[0]["player"])
	assert.Equal(t, "262", resp.Players[3]["ping"])
	assert.Equal(t, gs1.VerVanilla, resp.Version)
}

func TestQuery_AdminModServerQueryIsSupported(t *testing.T) {
	responses := make(chan []byte)
	go func() {
		responses <- b(
			"\\statusresponse\\0\\hostname\\[C=FF0000][c=33CCCC]>|S[C=FFFFFF]S|<[c=ffff00]Arg[C=ffffff]en[c=33CCCC]tina\xae[c=ff0000]-By FNXgaming.com" + // nolint: lll
				"\\numplayers\\10\\maxplayers\\16\\gametype\\Barricaded Suspects\\gamevariant\\SWAT 4\\" +
				"mapname\\A-Bomb Nightclub\\hostport\\10780\\password\\0\\gamever\\1.0\\statsenabled\\0" +
				"\\swatwon\\2\\suspectswon\\0\\round\\3\\numrounds\\3\\player_0\\darwinn\\player_1\\kyle" +
				"\\player_2\\super\\player_3\\\xab|FAL|cucuso\\player_4\\||AT||Lp!\\player_5\\Diejack1" +
				"\\player_6\\Player1232\\player_7\\Mojojojo\\player_8\\DrLemonn\\player_9\\elmatap\\score_0\\4\\eof\\",
		)
		responses <- b(
			"\\statusresponse\\1\\score_1\\2\\score_2\\1\\score_3\\10\\score_4\\14\\score_5\\-3\\score_6\\11" +
				"\\score_7\\25\\score_8\\18\\score_9\\5\\ping_0\\67\\ping_1\\184\\ping_2\\265\\ping_3\\255" +
				"\\ping_4\\54\\ping_5\\218\\ping_6\\208\\ping_7\\136\\ping_8\\70\\ping_9\\64\\team_0\\0\\team_1\\0" +
				"\\team_2\\1\\team_3\\0\\team_4\\1\\team_5\\0\\team_6\\1\\team_7\\1\\team_8\\0\\team_9\\0\\kills_0\\4" +
				"\\kills_1\\2\\kills_2\\1\\kills_3\\5\\kills_4\\14\\kills_5\\3\\kills_6\\6\\kills_7\\10\\kills_8\\8" +
				"\\kills_9\\6\\tkills_5\\2\\tkills_9\\2\\deaths_0\\6\\deaths_1\\9" +
				"\\deaths_2\\4\\deaths_3\\4\\deaths_4\\8\\deaths_5\\4\\deaths_6\\7\\eof\\",
		)
		responses <- b(
			"\\statusresponse\\2\\deaths_7\\5\\deaths_8\\7\\deaths_9\\4" +
				"\\arrests_3\\1\\arrests_6\\1\\arrests_7\\3\\arrests_8\\2\\arrests_9" +
				"\\1\\arrested_1\\1\\arrested_2\\2\\arrested_4\\1\\arrested_5\\1\\arrested_6\\1" +
				"\\arrested_9\\2\\queryid\\AMv1\\final\\\\eof\\",
		)
	}()
	server, cancel := gs1.PrepareGS1Server(responses)
	defer cancel()
	resp, err := gs1.Query(context.TODO(), server.LocalAddrPort(), time.Millisecond*10)
	assert.NoError(t, err)
	assert.Equal(t,
		"[C=FF0000][c=33CCCC]>|S[C=FFFFFF]S|<[c=ffff00]Arg[C=ffffff]en[c=33CCCC]tina®[c=ff0000]-By FNXgaming.com",
		resp.Fields["hostname"],
	)
	assert.Equal(t, "10780", resp.Fields["hostport"])
	assert.Equal(t, "10", resp.Fields["numplayers"])
	assert.Equal(t, "AMv1", resp.Fields["queryid"])
	assert.Contains(t, resp.Fields, "final")
	assert.Len(t, resp.Players, 10)
	assert.Equal(t, "«|FAL|cucuso", resp.Players[3]["player"])
	assert.Equal(t, "14", resp.Players[4]["kills"])
	assert.Equal(t, "3", resp.Players[5]["kills"])
	assert.Equal(t, "2", resp.Players[8]["arrests"])
	assert.Equal(t, "67", resp.Players[0]["ping"])
	assert.Equal(t, "1", resp.Players[2]["score"])
	assert.Equal(t, gs1.VerAM, resp.Version)
}

func TestQuery_AdminModSplitServerQueryIsSupported(t *testing.T) {
	responses := make(chan []byte)
	go func() {
		// last packet comes first and so forth
		responses <- b(
			"\\statusresponse\\2\\kills_13\\1\\kills_14\\1\\deaths_1\\1\\deaths_2\\1\\deaths_4\\1\\deaths_5\\1" +
				"\\deaths_9\\1\\deaths_14\\1\\queryid\\AMv1\\final\\\\eof\\",
		)
		// key, value of score_0 from statusresponse=0 are split
		responses <- b(
			"\\statusresponse\\1\\0\\score_1\\0\\score_2\\1\\score_3\\0\\score_4\\0\\score_5\\0\\score_6\\0" +
				"\\score_7\\0\\score_8\\1\\score_9\\0\\score_10\\0\\score_11\\0\\score_12\\2\\score_13\\1" +
				"\\score_14\\1\\ping_0\\155\\ping_1\\127\\ping_2\\263\\ping_3\\163\\ping_4\\111\\ping_5\\117\\ping_6" +
				"\\142\\ping_7\\121\\ping_8\\159\\ping_9\\142\\ping_10\\72\\ping_11\\154\\ping_12\\212\\ping_13" +
				"\\123\\ping_14\\153\\team_0\\1\\team_1\\0\\team_2\\1\\team_3\\0\\team_4\\0\\team_5\\0\\team_6\\1" +
				"\\team_7\\1\\team_8\\0\\team_9\\0\\team_10\\0\\team_11\\1\\team_12\\1\\team_13\\0\\team_14\\1" +
				"\\kills_2\\1\\kills_8\\1\\kills_12\\2\\eof\\",
		)
		responses <- b(
			"\\statusresponse\\0\\hostname\\{FAB} Clan Server\\numplayers\\15\\maxplayers" +
				"\\16\\gametype\\VIP Escort\\gamevariant\\SWAT 4\\mapname\\Red Library Offices" +
				"\\hostport\\10580\\password\\0\\gamever\\1.0\\statsenabled\\0\\swatwon\\1\\suspectswon\\0" +
				"\\round\\2\\numrounds\\7\\player_0\\{FAB}Nikki_Sixx<CPL>\\player_1\\Nico^Elite\\player_2" +
				"\\Balls\\player_3\\\xab|FAL|\xdc\xee\xee\xe4^\\player_4\\Reynolds\\player_5\\4Taws\\player_6" +
				"\\Daro\\player_7\\Majos\\player_8\\mi\\player_9\\tony\\player_10\\MENDEZ\\player_11\\ARoXDeviL" +
				"\\player_12\\{FAB}Chry<CPL>\\player_13\\P\\player_14\\xXx\\score_0\\eof\\",
		)
	}()
	server, cancel := gs1.PrepareGS1Server(responses)
	defer cancel()
	resp, err := gs1.Query(context.TODO(), server.LocalAddrPort(), time.Millisecond*10)
	assert.NoError(t, err)
	assert.Equal(t, "{FAB} Clan Server", resp.Fields["hostname"])
	assert.Equal(t, "10580", resp.Fields["hostport"])
	assert.Equal(t, "VIP Escort", resp.Fields["gametype"])
	assert.Equal(t, "AMv1", resp.Fields["queryid"])
	assert.Contains(t, resp.Fields, "final")
	assert.Len(t, resp.Players, 15)
	assert.Equal(t, "0", resp.Players[0]["score"])
	assert.Equal(t, "163", resp.Players[3]["ping"])
	assert.Equal(t, "«|FAL|Üîîä^", resp.Players[3]["player"])
	assert.Equal(t, "2", resp.Players[12]["kills"])
	assert.Equal(t, "P", resp.Players[13]["player"])
	assert.Equal(t, gs1.VerAM, resp.Version)
}

func TestQuery_GS1ModServerQueryIsSupported(t *testing.T) {
	responses := make(chan []byte)
	go func() {
		// last packet comes first and so forth
		responses <- b(
			"\\player_3\\Morgan\\score_3\\6\\ping_3\\53\\team_3\\1\\kills_3\\6\\deaths_3\\7" +
				"\\arrested_3\\1\\player_4\\Jericho\\score_4\\3\\ping_4\\46\\team_4\\0\\kills_4\\3" +
				"\\deaths_4\\12\\player_5\\Bolint\\score_5\\21\\ping_5\\57\\team_5\\1\\kills_5\\16" +
				"\\deaths_5\\8\\arrests_5\\1\\player_6\\FsB\\score_6\\2\\ping_6\\46\\team_6\\1\\kills_6\\5" +
				"\\deaths_6\\10\\tkills_6\\1\\arrested_6\\1\\player_7\\t00naab\\score_7\\11\\ping_7\\27" +
				"\\team_7\\0\\kills_7\\11\\vip_7\\1\\player_8\\ob\\score_8\\2\\ping_8\\74\\team_8\\1" +
				"\\kills_8\\2\\deaths_8\\3\\player_9\\martino\\score_9\\5\\ping_9\\67\\team_9\\1\\queryid\\2",
		)
		// key, value of score_0 from statusresponse=0 are split
		responses <- b(
			"\\hostname\\-==MYT Team Svr==-\\numplayers\\13\\maxplayers\\16" +
				"\\gametype\\VIP Escort\\gamevariant\\SWAT 4\\mapname\\Fairfax Residence" +
				"\\hostport\\10480\\password\\false\\gamever\\1.1\\round\\5\\numrounds\\5" +
				"\\timeleft\\286\\timespecial\\0\\swatscore\\41\\suspectsscore\\36\\swatwon" +
				"\\1\\suspectswon\\2\\player_0\\ugatz\\score_0\\0\\ping_0\\43\\team_0\\1" +
				"\\deaths_0\\9\\player_1\\|CSI|Miami\\score_1\\8\\ping_1\\104\\team_1\\0" +
				"\\kills_1\\8\\deaths_1\\4\\player_2\\aphawil\\score_2\\7\\ping_2\\69" +
				"\\team_2\\0\\kills_2\\8\\deaths_2\\11\\tkills_2\\2\\arrests_2\\1\\queryid\\1",
		)
		responses <- b(
			"\\kills_9\\5\\deaths_9\\2\\player_10\\conoeMadre\\score_10\\7\\ping_10\\135\\team_10\\0" +
				"\\kills_10\\7\\deaths_10\\2\\player_11\\Enigma51\\score_11\\0\\ping_11\\289\\team_11\\0" +
				"\\deaths_11\\1\\player_12\\Billy\\score_12\\0\\ping_12\\999\\team_12\\0\\queryid\\3\\final\\",
		)
	}()
	server, cancel := gs1.PrepareGS1Server(responses)
	defer cancel()
	resp, err := gs1.Query(context.TODO(), server.LocalAddrPort(), time.Millisecond*10)
	assert.NoError(t, err)
	assert.Equal(t, "-==MYT Team Svr==-", resp.Fields["hostname"])
	assert.Equal(t, "13", resp.Fields["numplayers"])
	assert.Equal(t, "36", resp.Fields["suspectsscore"])
	assert.Equal(t, "286", resp.Fields["timeleft"])
	assert.Equal(t, "2", resp.Fields["suspectswon"])
	assert.Equal(t, "10480", resp.Fields["hostport"])
	assert.Equal(t, "VIP Escort", resp.Fields["gametype"])
	assert.Contains(t, resp.Fields, "final")
	assert.Len(t, resp.Players, 13)
	assert.Equal(t, "0", resp.Players[12]["score"])
	assert.Equal(t, "Morgan", resp.Players[3]["player"])
	assert.Equal(t, "999", resp.Players[12]["ping"])
	assert.Equal(t, gs1.VerGS1, resp.Version)
}

func TestQuery_GS1ModServerObjectivesAreSupported(t *testing.T) {
	tests := []struct {
		name       string
		packets    [][]byte
		objectives []string
	}{
		{
			"single packet",
			[][]byte{
				b("\\hostname\\-==MYT Co-op Svr==-\\numplayers\\0\\maxplayers\\5\\gametype\\CO-OP" +
					"\\gamevariant\\SWAT 4\\mapname\\DuPlessis Diamond Center\\hostport\\10880\\password\\false" +
					"\\gamever\\1.1\\round\\1\\numrounds\\1\\timeleft\\316\\timespecial\\0" +
					"\\obj_Neutralize_All_Enemies\\0\\obj_Rescue_All_Hostages\\0\\tocreports\\0/11" +
					"\\weaponssecured\\0/0\\queryid\\1\\final\\",
				),
			},
			[]string{"Neutralize_All_Enemies", "Rescue_All_Hostages"},
		},
		{
			"multiple packets",
			[][]byte{
				b("\\hostname\\[c=0099ff]SEF 7.0 EU [c=ffffff]www.swat4.tk\\numplayers\\2\\maxplayers\\10" +
					"\\gametype\\CO-OP\\gamevariant\\SEF\\mapname\\Mt. Threshold Research Center" +
					"\\hostport\\10480\\password\\false\\gamever\\7.0\\round\\1\\numrounds\\1\\timeleft\\0" +
					"\\timespecial\\0\\obj_Neutralize_All_Enemies\\0\\obj_Rescue_All_Hostages\\0\\queryid\\1",
				),
				b("\\obj_Rescue_Sterling\\0\\obj_Neutralize_TerrorLeader\\0\\obj_Secure_Briefcase\\0" +
					"\\tocreports\\21/25\\weaponssecured\\5/8\\player_0\\Soup\\score_0\\0\\ping_0\\65" +
					"\\team_0\\0\\coopstatus_0\\2\\player_1\\McDuffin\\score_1\\0\\ping_1\\90\\team_1\\0" +
					"\\coopstatus_1\\0\\queryid\\2\\final\\",
				),
			},
			[]string{
				"Neutralize_All_Enemies", "Rescue_All_Hostages",
				"Rescue_Sterling", "Neutralize_TerrorLeader", "Secure_Briefcase",
			},
		},
		{
			"multiple reversed packets",
			[][]byte{
				b("\\obj_Rescue_Sterling\\0\\obj_Neutralize_TerrorLeader\\0\\obj_Secure_Briefcase\\0" +
					"\\tocreports\\21/25\\weaponssecured\\5/8\\player_0\\Soup\\score_0\\0\\ping_0\\65" +
					"\\team_0\\0\\coopstatus_0\\2\\player_1\\McDuffin\\score_1\\0\\ping_1\\90\\team_1\\0" +
					"\\coopstatus_1\\0\\queryid\\2\\final\\",
				),
				b("\\hostname\\[c=0099ff]SEF 7.0 EU [c=ffffff]www.swat4.tk\\numplayers\\2\\maxplayers\\10" +
					"\\gametype\\CO-OP\\gamevariant\\SEF\\mapname\\Mt. Threshold Research Center" +
					"\\hostport\\10480\\password\\false\\gamever\\7.0\\round\\1\\numrounds\\1\\timeleft\\0" +
					"\\timespecial\\0\\obj_Neutralize_All_Enemies\\0\\obj_Rescue_All_Hostages\\0\\queryid\\1",
				),
			},
			[]string{
				"Neutralize_All_Enemies", "Rescue_All_Hostages",
				"Rescue_Sterling", "Neutralize_TerrorLeader", "Secure_Briefcase",
			},
		},
		{
			"incomplete field name",
			[][]byte{
				b("\\hostname\\-==MYT Co-op Svr==-\\numplayers\\0\\maxplayers\\5\\gametype\\CO-OP" +
					"\\gamevariant\\SWAT 4\\mapname\\DuPlessis Diamond Center\\hostport\\10880\\password\\false" +
					"\\gamever\\1.1\\round\\1\\numrounds\\1\\timeleft\\316\\timespecial\\0" +
					"\\obj_Neutralize_All_Enemies\\0\\obj_\\0\\tocreports\\0/11" +
					"\\weaponssecured\\0/0\\queryid\\1\\final\\",
				),
			},
			[]string{"Neutralize_All_Enemies"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			responses := make(chan []byte)
			go func() {
				for _, packet := range tt.packets {
					responses <- packet
				}
			}()
			server, cancel := gs1.PrepareGS1Server(responses)
			defer cancel()
			resp, err := gs1.Query(context.TODO(), server.LocalAddrPort(), time.Millisecond*10)
			assert.NoError(t, err)
			objectives := make([]string, 0, len(resp.Objectives))
			for _, obj := range resp.Objectives {
				objectives = append(objectives, obj["name"])
			}
			assert.Equal(t, tt.objectives, objectives)
		})
	}

	responses := make(chan []byte)
	go func() {
		// last packet comes first and so forth
		responses <- b(
			"\\player_3\\Morgan\\score_3\\6\\ping_3\\53\\team_3\\1\\kills_3\\6\\deaths_3\\7" +
				"\\arrested_3\\1\\player_4\\Jericho\\score_4\\3\\ping_4\\46\\team_4\\0\\kills_4\\3" +
				"\\deaths_4\\12\\player_5\\Bolint\\score_5\\21\\ping_5\\57\\team_5\\1\\kills_5\\16" +
				"\\deaths_5\\8\\arrests_5\\1\\player_6\\FsB\\score_6\\2\\ping_6\\46\\team_6\\1\\kills_6\\5" +
				"\\deaths_6\\10\\tkills_6\\1\\arrested_6\\1\\player_7\\t00naab\\score_7\\11\\ping_7\\27" +
				"\\team_7\\0\\kills_7\\11\\vip_7\\1\\player_8\\ob\\score_8\\2\\ping_8\\74\\team_8\\1" +
				"\\kills_8\\2\\deaths_8\\3\\player_9\\martino\\score_9\\5\\ping_9\\67\\team_9\\1\\queryid\\2",
		)
		// key, value of score_0 from statusresponse=0 are split
		responses <- b(
			"\\hostname\\-==MYT Team Svr==-\\numplayers\\13\\maxplayers\\16" +
				"\\gametype\\VIP Escort\\gamevariant\\SWAT 4\\mapname\\Fairfax Residence" +
				"\\hostport\\10480\\password\\false\\gamever\\1.1\\round\\5\\numrounds\\5" +
				"\\timeleft\\286\\timespecial\\0\\swatscore\\41\\suspectsscore\\36\\swatwon" +
				"\\1\\suspectswon\\2\\player_0\\ugatz\\score_0\\0\\ping_0\\43\\team_0\\1" +
				"\\deaths_0\\9\\player_1\\|CSI|Miami\\score_1\\8\\ping_1\\104\\team_1\\0" +
				"\\kills_1\\8\\deaths_1\\4\\player_2\\aphawil\\score_2\\7\\ping_2\\69" +
				"\\team_2\\0\\kills_2\\8\\deaths_2\\11\\tkills_2\\2\\arrests_2\\1\\queryid\\1",
		)
		responses <- b(
			"\\kills_9\\5\\deaths_9\\2\\player_10\\conoeMadre\\score_10\\7\\ping_10\\135\\team_10\\0" +
				"\\kills_10\\7\\deaths_10\\2\\player_11\\Enigma51\\score_11\\0\\ping_11\\289\\team_11\\0" +
				"\\deaths_11\\1\\player_12\\Billy\\score_12\\0\\ping_12\\999\\team_12\\0\\queryid\\3\\final\\",
		)
	}()
	server, cancel := gs1.PrepareGS1Server(responses)
	defer cancel()
	resp, err := gs1.Query(context.TODO(), server.LocalAddrPort(), time.Millisecond*10)
	assert.NoError(t, err)
	assert.Equal(t, "-==MYT Team Svr==-", resp.Fields["hostname"])
	assert.Equal(t, "13", resp.Fields["numplayers"])
	assert.Equal(t, "36", resp.Fields["suspectsscore"])
	assert.Equal(t, "286", resp.Fields["timeleft"])
	assert.Equal(t, "2", resp.Fields["suspectswon"])
	assert.Equal(t, "10480", resp.Fields["hostport"])
	assert.Equal(t, "VIP Escort", resp.Fields["gametype"])
	assert.Contains(t, resp.Fields, "final")
	assert.Len(t, resp.Players, 13)
	assert.Equal(t, "0", resp.Players[12]["score"])
	assert.Equal(t, "Morgan", resp.Players[3]["player"])
	assert.Equal(t, "999", resp.Players[12]["ping"])
	assert.Equal(t, gs1.VerGS1, resp.Version)
}

func TestQuery_QueryIDMayBeNotInteger(t *testing.T) {
	tests := []struct {
		name    string
		packets [][]byte
	}{
		{
			"gs",
			[][]byte{
				b("\\hostname\\test\\hostport\\10480\\queryid\\gs1\\final\\"),
			},
		},
		{
			"1.1",
			[][]byte{
				b("\\hostname\\test\\hostport\\10480\\queryid\\1.1\\final\\"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			responses := make(chan []byte)
			go func() {
				for _, packet := range tt.packets {
					responses <- packet
				}
			}()
			server, cancel := gs1.PrepareGS1Server(responses)
			defer cancel()
			resp, err := gs1.Query(context.TODO(), server.LocalAddrPort(), time.Millisecond*10)
			assert.NoError(t, err)
			assert.Contains(t, resp.Fields, "final")
			assert.Equal(t, "", resp.Fields["final"])
			assert.Equal(t, "10480", resp.Fields["hostport"])
			assert.Equal(t, "test", resp.Fields["hostname"])
			assert.Equal(t, gs1.VerVanilla, resp.Version)
		})
	}
}

func TestQuery_StatusResponseIsZeroBased(t *testing.T) {
	responses := make(chan []byte)
	go func() {
		responses <- b("\\statusresponse\\0\\hostname\\test\\queryid\\AMv1\\eof\\")
		responses <- b("\\statusresponse\\1\\hostport\\10480\\queryid\\AMv1\\final\\\\eof\\")
	}()
	server, cancel := gs1.PrepareGS1Server(responses)
	defer cancel()
	resp, err := gs1.Query(context.TODO(), server.LocalAddrPort(), time.Millisecond*10)
	assert.NoError(t, err)
	assert.Contains(t, resp.Fields, "final")
	assert.Equal(t, "", resp.Fields["final"])
	assert.Equal(t, "10480", resp.Fields["hostport"])
	assert.Equal(t, "test", resp.Fields["hostname"])
	assert.Equal(t, gs1.VerAM, resp.Version)
}

func TestQuery_VariablePacketOrder(t *testing.T) {
	tests := []struct {
		name    string
		packets [][]byte
	}{
		{
			"normal order",
			[][]byte{
				b("\\statusresponse\\0\\hostname\\test\\queryid\\AMv1\\eof\\"),
				b("\\statusresponse\\1\\hostport\\10480\\queryid\\AMv1\\final\\\\eof\\"),
			},
		},
		{
			"reversed order",
			[][]byte{
				b("\\statusresponse\\1\\hostport\\10480\\queryid\\AMv1\\final\\\\eof\\"),
				b("\\statusresponse\\0\\hostname\\test\\queryid\\AMv1\\eof\\"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			responses := make(chan []byte)
			go func() {
				for _, packet := range tt.packets {
					responses <- packet
				}
			}()
			server, cancel := gs1.PrepareGS1Server(responses)
			defer cancel()
			resp, err := gs1.Query(context.TODO(), server.LocalAddrPort(), time.Millisecond*10)
			assert.NoError(t, err)
			assert.Contains(t, resp.Fields, "final")
			assert.Equal(t, "", resp.Fields["final"])
			assert.Equal(t, "10480", resp.Fields["hostport"])
			assert.Equal(t, "test", resp.Fields["hostname"])
		})
	}
}

func TestQuery_NoProperResponse(t *testing.T) {
	tests := []struct {
		name    string
		packets [][]byte
		wantErr error
	}{
		{
			"no packets",
			nil,
			os.ErrDeadlineExceeded,
		},
		{
			"no final",
			[][]byte{
				b("\\hostname\\test\\queryid\\1"),
				b("\\hostport\\10480\\queryid\\2"),
			},
			os.ErrDeadlineExceeded,
		},
		{
			"no packet order",
			[][]byte{
				b("\\hostname\\test\\hostport\\10480\\final\\"),
			},
			gs1.ErrResponseMalformed,
		},
		{
			"inconsistent order #1",
			[][]byte{
				b("\\hostname\\test\\queryid\\1"),
				b("\\hostport\\10480\\queryid\\2\\"),
				b("\\gametype\\VIP Escort\\queryid\\4\\final\\"),
			},
			os.ErrDeadlineExceeded,
		},
		{
			"inconsistent order #2",
			[][]byte{
				b("\\hostname\\test\\queryid\\2"),
				b("\\hostport\\10480\\queryid\\3\\"),
				b("\\gametype\\VIP Escort\\queryid\\4\\final\\"),
			},
			os.ErrDeadlineExceeded,
		},
		{
			"queryid cannot be zero",
			[][]byte{
				b("\\hostname\\test\\queryid\\0"),
				b("\\hostport\\10480\\queryid\\1\\final\\"),
			},
			gs1.ErrResponseMalformed,
		},
		{
			"queryid cannot be negative",
			[][]byte{
				b("\\hostname\\test\\queryid\\-1"),
				b("\\hostport\\10480\\queryid\\0\\"),
				b("\\gametype\\VIP Escort\\queryid\\1\\final\\"),
			},
			gs1.ErrResponseMalformed,
		},
		{
			"statusresponse is zero based",
			[][]byte{
				b("\\statusresponse\\1\\hostname\\test\\queryid\\AMv1\\eof\\"),
				b("\\statusresponse\\2\\hostport\\10480\\queryid\\AMv1\\final\\\\eof\\"),
			},
			os.ErrDeadlineExceeded,
		},
		{
			"invalid player id",
			[][]byte{
				b("\\statusresponse\\0\\hostname\\[c=ffff00]WWW.EPiCS.TOP\\numplayers\\6\\maxplayers\\16" +
					"\\gametype\\Barricaded Suspects\\gamevariant\\SWAT 4X\\mapname\\The Wolcott Projects" +
					"\\hostport\\10480\\password\\0\\gamever\\1.0\\statsenabled\\0\\swatwon\\0\\suspectswon\\0" +
					"\\round\\1\\numrounds\\3\\nextmap\\MP-FoodWall\\timeleft\\18\\swatscore\\0\\suspectsscore\\0" +
					"\\player_0\\[c=ffff00]op\\player_1\\[c=2F4F4F]AsD\\player_2\\mr\\player_3\\|Vx|Bogdy" +
					"\\player_4\\unknow\\player_FOO\\|{|BATMAN|}|\\score_0\\0\\score_1\\0\\score_2\\0\\score_3" +
					"\\0\\score_4\\0\\score_5\\0\\ping_0\\105\\ping_1\\125\\eof\\"),
				b("\\statusresponse\\1\\ping_2\\79\\ping_3\\72\\ping_4\\65\\ping_5\\81\\team_0\\1" +
					"\\team_1\\0\\team_2\\1\\team_3\\0\\team_4\\0\\team_5\\1\\queryid\\AMv1\\final\\\\eof\\"),
			},
			gs1.ErrResponseMalformed,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			responses := make(chan []byte)
			go func(packets [][]byte) {
				for _, packet := range packets {
					responses <- packet
				}
			}(tt.packets)
			server, cancel := gs1.PrepareGS1Server(responses)
			defer cancel()
			_, err := gs1.Query(context.TODO(), server.LocalAddrPort(), time.Millisecond*50)
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestQuery_ReadTimeout(t *testing.T) {
	tests := []struct {
		name    string
		delay   time.Duration
		wantErr error
	}{
		{
			"read timeout",
			time.Millisecond * 100,
			os.ErrDeadlineExceeded,
		},
		{
			"no timeout",
			time.Millisecond * 25,
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			responses := make(chan []byte)
			go func(delay time.Duration) {
				responses <- b("\\statusresponse\\0\\hostname\\test\\queryid\\AMv1\\eof\\")
				<-time.After(delay)
				responses <- b("\\statusresponse\\1\\hostport\\10480\\queryid\\AMv1\\final\\\\eof\\")
			}(tt.delay)
			server, cancel := gs1.PrepareGS1Server(responses)
			defer cancel()
			_, err := gs1.Query(context.TODO(), server.LocalAddrPort(), time.Millisecond*50)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestQuery_ParentContextIsCancelled(t *testing.T) {
	responses := make(chan []byte)
	go func() {
		responses <- b("\\statusresponse\\0\\hostname\\test\\queryid\\AMv1\\eof\\")
		<-time.After(time.Millisecond * 50)
		responses <- b("\\statusresponse\\1\\hostport\\10480\\queryid\\AMv1\\final\\\\eof\\")
	}()
	server, cancel := gs1.PrepareGS1Server(responses)
	defer cancel()

	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		<-time.After(time.Millisecond * 25)
		cancelCtx()
	}()

	_, err := gs1.Query(ctx, server.LocalAddrPort(), time.Millisecond*100)
	assert.ErrorIs(t, err, os.ErrDeadlineExceeded)
}
