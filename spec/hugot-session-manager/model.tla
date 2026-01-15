---- MODULE model ----
EXTENDS Integers, Sequences, TLC, FiniteSets

\* Constants for our model
CONSTANTS 
    Clients,        \* Set of client processes
    Backends,       \* Set of available backends (e.g. {ONNX, GO})
    MaxRetries      \* Bound for checking liveness/termination

\* Variables representing the actual system state
VARIABLES 
    sm_sessions,    \* Map [Backend -> SessionStatus]
    sm_closed,      \* Boolean
    sm_mutex,       \* Mutex owner (or NULL)
    
    \* Variables representing client states
    pc,             \* Program counter for each client
    client_vars,    \* scratchpad for local variables (like 'err', 'session')
    
    \* "Ghost" variables for verification/monitoring
    session_life,   \* [Backend -> {Unitialized, Created, Destroyed}]
    active_inferences \* Set of clients currently "using" a session (executing inference)

\* Model specific definitions
NULL == "NULL"
StatusNone == "None"
StatusActive == "Active"

SessionUninitialized == "Uninitialized"
SessionCreated == "Created"
SessionDestroyed == "Destroyed"

vars == <<sm_sessions, sm_closed, sm_mutex, pc, client_vars, session_life, active_inferences>>

\* =========================================================================
\* Actions - Modeling the Go Code
\* =========================================================================

\* Defines where each client is in their workflow
\* "Idle" -> "CallGetSession" -> "InGetSession" -> "UseSession" -> "Idle"
\* Or "Idle" -> "CallClose" -> "InClose" -> "Idle"

Init ==
    /\ sm_sessions = [b \in Backends |-> StatusNone]
    /\ sm_closed = FALSE
    /\ sm_mutex = NULL
    /\ pc = [c \in Clients |-> "Idle"]
    /\ client_vars = [c \in Clients |-> [backend |-> NULL, session |-> NULL, err |-> NULL]]
    /\ session_life = [b \in Backends |-> SessionUninitialized]
    /\ active_inferences = {}

\* -------------------------------------------------------------------------
\* Helper: Acquire Lock
AcquireLock(c) ==
    /\ sm_mutex = NULL
    /\ sm_mutex' = c
    /\ UNCHANGED <<sm_sessions, sm_closed, client_vars, session_life, active_inferences>>

ReleaseLock(c) ==
    /\ sm_mutex = c
    /\ sm_mutex' = NULL
    /\ UNCHANGED <<sm_sessions, sm_closed, client_vars, session_life, active_inferences>>

\* -------------------------------------------------------------------------
\* Client Action: Start GetSession
StartGetSession(c, backend) ==
    /\ pc[c] = "Idle"
    /\ pc' = [pc EXCEPT ![c] = "EnterGetSession"]
    /\ client_vars' = [client_vars EXCEPT ![c].backend = backend, ![c].session = NULL, ![c].err = NULL]
    /\ UNCHANGED <<sm_sessions, sm_closed, sm_mutex, session_life, active_inferences>>

\* Client Action: Enter GetSession (Acquire Lock)
EnterGetSession(c) ==
    /\ pc[c] = "EnterGetSession"
    /\ AcquireLock(c)
    /\ pc' = [pc EXCEPT ![c] = "CheckClosed"]

\* Client Action: Check Closed
CheckClosed(c) ==
    /\ pc[c] = "CheckClosed"
    /\ IF sm_closed
        THEN 
            /\ client_vars' = [client_vars EXCEPT ![c].err = "ManagerClosed"]
            /\ pc' = [pc EXCEPT ![c] = "ExitGetSession"]
            /\ UNCHANGED <<session_life, sm_sessions>>
        ELSE
            /\ pc' = [pc EXCEPT ![c] = "CheckMap"]
            /\ UNCHANGED <<client_vars, session_life, sm_sessions>>
    /\ UNCHANGED <<sm_mutex, sm_closed, active_inferences>>

\* Client Action: Check Map and Maybe Create
CheckMap(c) ==
    /\ pc[c] = "CheckMap"
    /\ LET b == client_vars[c].backend IN
       IF sm_sessions[b] = StatusActive
       THEN
           \* Session exists
           /\ client_vars' = [client_vars EXCEPT ![c].session = b]
           /\ pc' = [pc EXCEPT ![c] = "ExitGetSession"]
           /\ UNCHANGED <<sm_sessions, session_life>>
       ELSE
           \* Create session
           \* Model assumption: CreateSession always succeeds for now
           /\ sm_sessions' = [sm_sessions EXCEPT ![b] = StatusActive]
           /\ session_life' = [session_life EXCEPT ![b] = SessionCreated]
           /\ client_vars' = [client_vars EXCEPT ![c].session = b]
           /\ pc' = [pc EXCEPT ![c] = "ExitGetSession"]
    /\ UNCHANGED <<sm_mutex, sm_closed, active_inferences>>

\* Client Action: Exit GetSession (Release Lock)
ExitGetSession(c) ==
    /\ pc[c] = "ExitGetSession"
    /\ ReleaseLock(c)
    /\ IF client_vars[c].err = NULL
        THEN pc' = [pc EXCEPT ![c] = "StartInference"]
        ELSE pc' = [pc EXCEPT ![c] = "Idle"]

\* -------------------------------------------------------------------------
\* Client Action: Use Session (Inference)
\* This models the "outside" usage of the session returned
StartInference(c) ==
    /\ pc[c] = "StartInference"
    /\ active_inferences' = active_inferences \cup {c}
    /\ pc' = [pc EXCEPT ![c] = "EndInference"]
    /\ UNCHANGED <<sm_sessions, sm_closed, sm_mutex, client_vars, session_life>>

EndInference(c) ==
    /\ pc[c] = "EndInference"
    /\ active_inferences' = active_inferences \ {c}
    /\ pc' = [pc EXCEPT ![c] = "Idle"]
    /\ UNCHANGED <<sm_sessions, sm_closed, sm_mutex, client_vars, session_life>>

\* -------------------------------------------------------------------------
\* Client Action: Close Manager
StartClose(c) ==
    /\ pc[c] = "Idle"
    /\ pc' = [pc EXCEPT ![c] = "EnterClose"]
    /\ UNCHANGED <<sm_sessions, sm_closed, sm_mutex, client_vars, session_life, active_inferences>>

EnterClose(c) ==
    /\ pc[c] = "EnterClose"
    /\ AcquireLock(c)
    /\ pc' = [pc EXCEPT ![c] = "DoClose"]

DoClose(c) ==
    /\ pc[c] = "DoClose"
    /\ IF sm_closed
       THEN 
           \* Already closed, do nothing
           /\ pc' = [pc EXCEPT ![c] = "ExitClose"]
           /\ UNCHANGED <<sm_sessions, sm_closed, session_life>>
       ELSE
           \* Reset sessions and close
           \* Mark all currently ACTIVE sessions as destroyed
           /\ session_life' = [b \in Backends |-> IF sm_sessions[b] = StatusActive THEN SessionDestroyed ELSE session_life[b]]
           /\ sm_sessions' = [b \in Backends |-> StatusNone]
           /\ sm_closed' = TRUE
           /\ pc' = [pc EXCEPT ![c] = "ExitClose"]
    /\ UNCHANGED <<sm_mutex, client_vars, active_inferences>>

ExitClose(c) ==
    /\ pc[c] = "ExitClose"
    /\ ReleaseLock(c)
    /\ pc' = [pc EXCEPT ![c] = "Idle"]
    /\ UNCHANGED <<client_vars>>

\* =========================================================================
\* Temporal Properties & Invariants
\* =========================================================================

\* NEXT state relation
Next ==
    \/ \E c \in Clients, b \in Backends : StartGetSession(c, b)
    \/ \E c \in Clients : EnterGetSession(c)
    \/ \E c \in Clients : CheckClosed(c)
    \/ \E c \in Clients : CheckMap(c)
    \/ \E c \in Clients : ExitGetSession(c)
    \/ \E c \in Clients : StartInference(c)
    \/ \E c \in Clients : EndInference(c)
    \/ \E c \in Clients : StartClose(c)
    \/ \E c \in Clients : EnterClose(c)
    \/ \E c \in Clients : DoClose(c)
    \/ \E c \in Clients : ExitClose(c)

\* Invariant: No Use After Free
\* If a client is inferencing with a session, that session must NOT be destroyed.
UseAfterFree ==
    \A c \in active_inferences : 
        LET b == client_vars[c].session IN
        session_life[b] # SessionDestroyed

\* Invariant: Only one active session per backend (Singleton)
\* This is trivially true by our Map model, but let's check strict "Created" count if we tracked it differently.
\* For now, 'sm_sessions' map key uniqueness guarantees this. 

Spec == Init /\ [][Next]_vars

====
