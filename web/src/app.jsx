import Router from 'preact-router'
import { AuthProvider, LoginGate } from './auth.jsx'
import { ToastProvider } from './components/toast.jsx'
import { Layout } from './components/layout.jsx'
import { Alerts } from './pages/alerts.jsx'
import { AlertDetail } from './pages/alert-detail.jsx'
import { Services } from './pages/services.jsx'
import { ServiceDetail } from './pages/service-detail.jsx'
import { EscalationPolicies } from './pages/escalation-policies.jsx'
import { EscalationPolicyDetail } from './pages/escalation-policy-detail.jsx'
import { Schedules } from './pages/schedules.jsx'
import { ScheduleDetail } from './pages/schedule-detail.jsx'
import { Users } from './pages/users.jsx'
import { Incidents } from './pages/incidents.jsx'
import { IncidentDetail } from './pages/incident-detail.jsx'
import { Profile } from './pages/profile.jsx'
import { InviteAccept } from './pages/invite.jsx'
import { Teams } from './pages/teams.jsx'
import { TeamDetail } from './pages/team-detail.jsx'

export function App() {
  // Invite page is public (no auth required)
  const path = typeof window !== 'undefined' ? window.location.pathname : '/'
  if (path.startsWith('/invite/')) {
    const token = path.replace('/invite/', '')
    return <InviteAccept token={token} />
  }

  return (
    <AuthProvider>
      <LoginGate>
        <ToastProvider>
        <Layout>
          <Router>
            <Alerts path="/" />
            <Alerts path="/alerts" />
            <AlertDetail path="/alerts/:id" />
            <Services path="/services" />
            <ServiceDetail path="/services/:id" />
            <EscalationPolicies path="/escalation-policies" />
            <EscalationPolicyDetail path="/escalation-policies/:id" />
            <Schedules path="/schedules" />
            <ScheduleDetail path="/schedules/:id" />
            <Teams path="/teams" />
            <TeamDetail path="/teams/:id" />
            <Users path="/users" />
            <Incidents path="/incidents" />
            <IncidentDetail path="/incidents/:id" />
            <Profile path="/profile" />
          </Router>
        </Layout>
        </ToastProvider>
      </LoginGate>
    </AuthProvider>
  )
}
