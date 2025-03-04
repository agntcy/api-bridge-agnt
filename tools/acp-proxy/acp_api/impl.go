package acp_api

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

type ACPServer struct {
	Runs     map[string]Run // runID -> Run
	RunsLock sync.RWMutex

	Outputs     map[string]RunResult // runID -> RunOutput
	OutputsLock sync.RWMutex
}

func NewACPServer() *ACPServer {
	return &ACPServer{
		Runs:     make(map[string]Run),
		RunsLock: sync.RWMutex{},

		Outputs:     make(map[string]RunResult),
		OutputsLock: sync.RWMutex{},
	}
}

// ensure that we've conformed to the `ServerInterface` with a compile-time check
var _ ServerInterface = (*ACPServer)(nil)

func notImplemented(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`This method is not implemented 😢`))
}

// Search Agents
// (POST /agents/search)
func (*ACPServer) SearchAgents(w http.ResponseWriter, r *http.Request) {
	notImplemented(w)
}

// Get Agent
// (GET /agents/{agent_id})
func (*ACPServer) GetAgentByID(w http.ResponseWriter, r *http.Request, agentId openapi_types.UUID) {
	notImplemented(w)
}

// Get Agent Manifest from its id
// (GET /agents/{agent_id}/manifest)
func (*ACPServer) GetAgentManifestById(w http.ResponseWriter, r *http.Request, agentId openapi_types.UUID) {
	notImplemented(w)
}

// Create Background Run
// (POST /runs)
func (s *ACPServer) CreateRun(w http.ResponseWriter, r *http.Request) {
	var runToCreate CreateRunJSONRequestBody
	if err := json.NewDecoder(r.Body).Decode(&runToCreate); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("Creating run for agent ID %s", runToCreate.AgentId)
	runID := openapi_types.UUID{}
	createdRun := Run{
		AgentId:   runToCreate.AgentId,
		CreatedAt: time.Time{},
		Creation:  RunCreate{},
		RunId:     runID,
		Status:    "",
		ThreadId:  &openapi_types.UUID{},
		UpdatedAt: time.Time{},
	}

	s.RunsLock.Lock()
	defer s.RunsLock.Unlock()
	s.Runs[runID.String()] = createdRun

	log.Printf("Run created with ID %s", runID.String())

	timer := time.NewTimer(5 * time.Second)
	go func() {
		<-timer.C

		upstreamResponse, err := http.Get("https://httpbin.org/get")
		if err != nil {
			log.Printf("Error querying httpbin.org: %v", err)
		} else {
			defer upstreamResponse.Body.Close()
			log.Printf("Successfully queried httpbin.org, status: %s", upstreamResponse.Status)
		}
		body, err := io.ReadAll(upstreamResponse.Body)
		if err != nil {
			log.Printf("Error reading response body: %v", err)
		}

		acpResult := OutputSchema{
			"response": string(body),
		}
		s.OutputsLock.Lock()
		s.Outputs[runID.String()] = RunResult{
			Result: acpResult,
			RunId:  runID,
			Status: RunStatusSuccess,
			Type:   Result,
		}
		s.OutputsLock.Unlock()

		log.Printf("Run output added for ID %s", runID.String())
	}()

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(createdRun)
}

// Search Runs
// (POST /runs/search)
func (*ACPServer) SearchRuns(w http.ResponseWriter, r *http.Request) {
	notImplemented(w)
}

// Delete a run. If running, cancel and then delete.
// (DELETE /runs/{run_id})
func (s *ACPServer) DeleteRun(w http.ResponseWriter, r *http.Request, runId openapi_types.UUID) {
	s.RunsLock.Lock()
	defer s.RunsLock.Unlock()

	_, exists := s.Runs[runId.String()]
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	delete(s.Runs, runId.String())
	log.Printf("Run deleted with ID %s", runId.String())

	w.WriteHeader(http.StatusOK)
}

// Get a previously created Run
// (GET /runs/{run_id})
func (s *ACPServer) GetRun(w http.ResponseWriter, r *http.Request, runId openapi_types.UUID) {
	s.RunsLock.RLock()
	defer s.RunsLock.RUnlock()

	run, exists := s.Runs[runId.String()]
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	_ = json.NewEncoder(w).Encode(run)
}

// Resume an interrupted Run
// (POST /runs/{run_id})
func (*ACPServer) ResumeRun(w http.ResponseWriter, r *http.Request, runId openapi_types.UUID) {
	notImplemented(w)
}

// Retrieve last output of a run if available
// (GET /runs/{run_id}/output)
func (s *ACPServer) GetRunOutput(w http.ResponseWriter, r *http.Request, runId openapi_types.UUID, params GetRunOutputParams) {
	s.RunsLock.RLock()
	defer s.RunsLock.RUnlock()

	_, exists := s.Runs[runId.String()]
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	s.OutputsLock.Lock()
	defer s.OutputsLock.Unlock()

	runOutput, exists := s.Outputs[runId.String()]
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	delete(s.Outputs, runId.String())

	_ = json.NewEncoder(w).Encode(runOutput)
}

// Stream the run output
// (GET /runs/{run_id}/stream)
func (*ACPServer) GetRunStream(w http.ResponseWriter, r *http.Request, runId openapi_types.UUID) {
	notImplemented(w)
}

// Retrieve the thread state at the end of the run
// (GET /runs/{run_id}/threadstate)
func (*ACPServer) GetRunThreadstate(w http.ResponseWriter, r *http.Request, runId openapi_types.UUID) {
	notImplemented(w)
}

// Create an empty Thread
// (POST /threads)
func (*ACPServer) CreateThread(w http.ResponseWriter, r *http.Request) {
	notImplemented(w)
}

// Search Threads
// (POST /threads/search)
func (*ACPServer) SearchThreads(w http.ResponseWriter, r *http.Request) {
	notImplemented(w)
}

// Delete a thread. If the thread contains any pending run, deletion fails.
// (DELETE /threads/{thread_id})
func (*ACPServer) DeleteThread(w http.ResponseWriter, r *http.Request, threadId openapi_types.UUID) {
	notImplemented(w)
}

// Get a previously created Thread
// (GET /threads/{thread_id})
func (*ACPServer) GetThread(w http.ResponseWriter, r *http.Request, threadId openapi_types.UUID) {
	notImplemented(w)
}

// Retrieve the list of runs and associated state at the end of each run.
// (GET /threads/{thread_id}/history)
func (*ACPServer) GetThreadHistory(w http.ResponseWriter, r *http.Request, threadId openapi_types.UUID) {
	notImplemented(w)
}

// Retrieve the current state associated with the thread
// (GET /threads/{thread_id}/state)
func (*ACPServer) GetThreadState(w http.ResponseWriter, r *http.Request, threadId openapi_types.UUID) {
	notImplemented(w)
}
