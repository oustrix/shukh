package main

import "testing"

// -origins принимают руками, через пробел после запятой — это нормальная форма записи
// списка. Без trim элемент " http://b" не совпадёт ни с одним Origin браузера, и
// allowlist молча перестаёт работать именно для второго и последующих origin.
func TestParseOriginsTrimsSpaces(t *testing.T) {
	got := parseOrigins("http://localhost:5173, http://127.0.0.1:5173 ")
	want := []string{"http://localhost:5173", "http://127.0.0.1:5173"}
	if len(got) != len(want) {
		t.Fatalf("parseOrigins = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("parseOrigins = %q, want %q", got, want)
		}
	}
}

// Пустой флаг и мусорные разделители не должны рождать пустой origin: он совпал бы
// с отсутствующим заголовком Origin в originAllowed.
func TestParseOriginsDropsEmpty(t *testing.T) {
	if got := parseOrigins(""); len(got) != 0 {
		t.Fatalf("parseOrigins(\"\") = %q, want empty", got)
	}
	if got := parseOrigins(" , ,"); len(got) != 0 {
		t.Fatalf("parseOrigins(\" , ,\") = %q, want empty", got)
	}
}
