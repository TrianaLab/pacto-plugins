package infer

// detectFastAPI checks dependency files for FastAPI.
func detectFastAPI(dir string) bool {
	return fileContains(dir, "requirements.txt", "fastapi") ||
		fileContains(dir, "pyproject.toml", "fastapi") ||
		fileContains(dir, "Pipfile", "fastapi") ||
		fileContains(dir, "setup.py", "fastapi") ||
		fileContains(dir, "setup.cfg", "fastapi")
}
