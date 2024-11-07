export function validateURL(url) {
	try {
		new URL(url); // Attempts to construct a URL object
		return true;
	} catch (e) {
		return false;
	}
}
