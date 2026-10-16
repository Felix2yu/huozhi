/** Luhn 校验 (信用卡号) */
export function luhnCheck(cardNo: string): boolean {
	const digits = cardNo.replace(/\D/g, '');
	if (digits.length < 13 || digits.length > 19) return false;
	let sum = 0;
	let shouldDouble = false;
	for (let i = digits.length - 1; i >= 0; i--) {
		let d = parseInt(digits[i]);
		if (shouldDouble) {
			d *= 2;
			if (d > 9) d -= 9;
		}
		sum += d;
		shouldDouble = !shouldDouble;
	}
	return sum % 10 === 0;
}

export function isValidEmail(email: string): boolean {
	return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email);
}

export function isValidPassword(pwd: string): boolean {
	return pwd.length >= 6;
}
