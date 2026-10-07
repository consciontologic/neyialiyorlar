# Re-export all stdlib calendar functionality to avoid shadowing
# This is necessary because our calendar package shadows the stdlib calendar module.
# When Python or 3rd-party libraries (like pandas via _strptime) try to import calendar,
# they get our package instead of the stdlib. We need to ensure all stdlib functionality
# is available through our package.
import sys
import importlib

# The trick: temporarily remove ourselves from sys.modules to import the stdlib calendar
_our_name = __name__
_our_module = sys.modules.pop(_our_name, None)

# Now import the stdlib calendar (without our package in the way)
_stdlib_calendar = importlib.import_module('calendar')

# Restore ourselves to sys.modules
if _our_module is not None:
    sys.modules[_our_name] = _our_module

# Re-export all stdlib calendar names into our namespace
for _attr in dir(_stdlib_calendar):
    if not _attr.startswith('_'):
        globals()[_attr] = getattr(_stdlib_calendar, _attr)

# Now add our custom calendar implementations
from .bist import BistCalendar

__all__ = ['BistCalendar']
