package conflict

// SetNotifier sets the conflict notifier for the service.
// Pass nil to disable notifications.
func (s *Service) SetNotifier(notifier ConflictNotifier) {
	s.notifier = notifier
}

// notifyConflictDetected fires the OnConflictDetected event if notifier is set.
func (s *Service) notifyConflictDetected(conflicts []ConflictInfo) {
	if s.notifier != nil {
		s.notifier.OnConflictDetected(conflicts)
	}
}

// notifyConflictResolved fires the OnConflictResolved event if notifier is set.
func (s *Service) notifyConflictResolved(file string) {
	if s.notifier != nil {
		s.notifier.OnConflictResolved(file)
	}
}

// notifyAllConflictsResolved fires the OnAllConflictsResolved event if notifier is set.
func (s *Service) notifyAllConflictsResolved() {
	if s.notifier != nil {
		s.notifier.OnAllConflictsResolved()
	}
}
