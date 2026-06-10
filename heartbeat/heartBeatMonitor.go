package heartbeat

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"syscall"
	"time"

	"interceptor-grpc/config"
	"interceptor-grpc/crController"

	"github.com/rs/zerolog/log"
)

// Timeout explícito: sem ele, um GET de health pendurado num backend saturado
// trava o loop do monitor (e lentidão viraria "falha" só quando a conexão
// caísse, de forma errática).
var hbClient = &http.Client{
	Timeout:   2 * time.Second,
	Transport: &http.Transport{DisableKeepAlives: true},
}

// flushGrace é a janela após um desbloqueio de tráfego pós-snapshot em que
// erros de APLICAÇÃO (status>299) no health não fecham o gate: o backend está
// digerindo o flush de backlog, não morto. Connection refused fecha SEMPRE
// (sinal inequívoco de pod morto, independe de graça).
const flushGrace = 60 * time.Second

func inFlushGrace() bool {
	t := crController.LastTrafficRelease.Load()
	return t > 0 && time.Since(time.Unix(0, t)) < flushGrace
}

// canaryKey é a chave reservada do canário de regressão de estado — fora do
// range usado pelos benchmarks/seeds (que ficam abaixo de ~2M).
const canaryKey = "999999999"

// lastCanary é o último valor que ESTE interceptor escreveu (ou adotou) no
// canário. Se a leitura regredir, o backend foi restaurado de um checkpoint
// antigo e o buffer pós-snapshot precisa de replay.
var lastCanary uint64

func Monitor() {
	// This function should be called in a go routine
	// It should monitor the heartbeat of the interceptor
	// If the interceptor is not responding, it should restart the interceptor
	path := config.GetHeartBeatPath()
	applicationURL := strings.TrimRight(config.GetApplicationURL(), "/")
	fullPath := applicationURL + "/" + path
	// Make a request to the interceptor
	numberRequestsFailed := 0
	numberRequestsSuccess := 0

	tick := time.Tick(5 * time.Second)
	for range tick {
		// #E: skip enquanto snapshot/restore esta acontecendo. CRIU congela o backend
		// durante o dump, fazendo /health retornar timeout/erro -- contar como falha
		// abriria o circuito falsamente.
		if crController.IsDoingSnapshot.Load() || crController.IsRestoringSnapshot.Load() {
			numberRequestsFailed = 0
			numberRequestsSuccess = 0
			continue
		}
		resp, err := hbClient.Get(fullPath)
		if err != nil {
			numberRequestsSuccess = 0
			if errors.Is(err, syscall.ECONNREFUSED) {
				// Pod morto de verdade (kube-proxy rejeita sem endpoints):
				// conta pra fechar o gate.
				numberRequestsFailed++
			}
			// Timeout/reset/etc: o backend pode estar só SATURADO (flush de
			// backlog) — fechar o gate aqui amplifica (mais backlog → flush
			// maior → mais timeout). Não conta pra fechar; o streak de sucesso
			// zerado já impede reabertura prematura.
			if numberRequestsFailed > 5 {
				crController.IsContainerUnavailable.Store(true)
			}
			continue
		}
		_, _ = io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode > 299 {
			numberRequestsSuccess = 0
			if !inFlushGrace() {
				// Erro de aplicação fora da janela de flush: conta.
				numberRequestsFailed++
			}
		} else {
			numberRequestsSuccess++
			numberRequestsFailed = 0
		}
		if numberRequestsFailed > 5 {
			crController.IsContainerUnavailable.Store(true)
		}
		if numberRequestsSuccess > 5 {
			// Transição indisponível -> disponível: só libera o tráfego. O
			// replay fica EXCLUSIVAMENTE com o canário: a transição dispara em
			// falso-positivo (flush de backlog derruba o /health sem restore
			// nenhum) e, num restore real, dispara DEPOIS do canário, re-
			// enfileirando o que o replay já re-registrou no buffer —
			// amplificação (medido: 173K do canário + 197K da transição).
			if crController.IsContainerUnavailable.Load() {
				log.Warn().Msg("Recovery detected by heartbeat: unblocking traffic (replay delegated to canary)")
			}
			crController.IsContainerUnavailable.Store(false)
		}

		// Canário de regressão: detector ÚNICO de restore. Todo restore real
		// regride o contador (escrito a cada tick, monotônico; o checkpoint é
		// sempre mais velho), inclusive quando o pod volta rápido demais pro
		// contador de falhas. Flush/overload não regride o canário => sem
		// falso-positivo.
		if numberRequestsFailed == 0 && !crController.IsContainerUnavailable.Load() {
			checkCanary(applicationURL)
		}
	}
}

// checkCanary lê o canário no backend e compara com o último valor escrito.
// Regressão (valor menor, ou chave sumida) => o backend foi restaurado de um
// checkpoint anterior => replay do buffer. Depois avança e regrava o canário.
func checkCanary(appURL string) {
	cur, found, err := canaryGet(appURL)
	if err != nil {
		return // backend instável neste tick; o contador de falhas cuida disso
	}
	if found {
		if lastCanary == 0 {
			// Interceptor (re)iniciou: adota o valor existente como base.
			lastCanary = cur
		} else if cur < lastCanary {
			stateRegressionRecovery()
		}
	} else if lastCanary > 0 {
		// Canário sumiu: restore pra um checkpoint anterior à sua criação.
		stateRegressionRecovery()
	}
	lastCanary++
	if err := canaryPost(appURL, lastCanary); err != nil {
		lastCanary-- // não conseguiu gravar: não avança a régua
	}
}

// stateRegressionRecovery bloqueia brevemente a admissão, re-enfileira o buffer
// pós-snapshot e libera — os replays drenam antes das requests novas.
func stateRegressionRecovery() {
	log.Warn().Uint64("last_canary", lastCanary).
		Msg("State regression detected (canary rolled back): backend restored from older checkpoint")
	crController.IsContainerUnavailable.Store(true)
	n := crController.ReplayBufferedRequests()
	crController.IsContainerUnavailable.Store(false)
	log.Warn().Int("replayed", n).Msg("State regression recovery: buffered requests queued for replay")
}

func canaryGet(appURL string) (uint64, bool, error) {
	resp, err := hbClient.Get(appURL + "/?key=" + canaryKey)
	if err != nil {
		return 0, false, err
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return 0, false, readErr
	}
	if resp.StatusCode == http.StatusNotFound {
		return 0, false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return 0, false, errBadStatus
	}
	v := strings.Trim(strings.TrimSpace(string(body)), "\"")
	n, parseErr := strconv.ParseUint(v, 10, 64)
	if parseErr != nil {
		// Valor ilegível: não dá pra raciocinar sobre regressão neste tick.
		return 0, false, parseErr
	}
	return n, true, nil
}

var errBadStatus = errBadStatusType{}

type errBadStatusType struct{}

func (errBadStatusType) Error() string { return "canary get: unexpected status" }

func canaryPost(appURL string, val uint64) error {
	form := url.Values{"key": {canaryKey}, "value": {strconv.FormatUint(val, 10)}}
	resp, err := hbClient.PostForm(appURL+"/", form)
	if err != nil {
		return err
	}
	_, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode > 299 {
		return errBadStatus
	}
	return nil
}
