"""
Pydantic models for request data validation
"""

import json
from typing import Any, Dict, List, Optional, Union

from pydantic import BaseModel, Field
from utils.setup_logging import logger

_original_json_encoder_default = getattr(json.JSONEncoder, "default")


class BaseModelWrapper(BaseModel):
    """Base model wrapper to ensure compatibility with TypedDict."""

    def get(self, key: str, default: Optional[Any] = None) -> Optional[Any]:
        """Get the value of a specific key."""
        return getattr(self, key, default)

    def to_dict(self) -> Dict[str, Any]:
        """Convert the model to a dictionary, handling nested Pydantic models."""
        return self.model_dump(exclude_none=True, mode="json")

    def to_json(self) -> str:
        """Convert the model to a JSON string."""
        return json.dumps(self.to_dict())
