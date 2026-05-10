package projectinit

import "testing"

func TestParseResourceAttrs_Empty(t *testing.T) {
	pairs := parseResourceAttrs("")
	if len(pairs) != 0 {
		t.Errorf("got %d pairs, want 0", len(pairs))
	}
}

func TestParseResourceAttrs_Multiple(t *testing.T) {
	pairs := parseResourceAttrs("enduser.id=jdoe,deployment.environment=prod")
	if len(pairs) != 2 {
		t.Fatalf("got %d pairs, want 2", len(pairs))
	}
	if pairs[0].Key != "enduser.id" || pairs[0].Value != "jdoe" {
		t.Errorf("pairs[0]=%+v", pairs[0])
	}
	if pairs[1].Key != "deployment.environment" || pairs[1].Value != "prod" {
		t.Errorf("pairs[1]=%+v", pairs[1])
	}
}

func TestParseResourceAttrs_TrimsSpaces(t *testing.T) {
	pairs := parseResourceAttrs("a=1, b=2 ,c =3")
	want := []struct{ k, v string }{{"a", "1"}, {"b", "2"}, {"c", "3"}}
	for i, w := range want {
		if pairs[i].Key != w.k || pairs[i].Value != w.v {
			t.Errorf("pairs[%d]=%+v, want {%q,%q}", i, pairs[i], w.k, w.v)
		}
	}
}

func TestParseResourceAttrs_SkipsMalformed(t *testing.T) {
	pairs := parseResourceAttrs("a=1,bogus,b=2")
	if len(pairs) != 2 || pairs[0].Key != "a" || pairs[1].Key != "b" {
		t.Errorf("got %+v, want a=1,b=2", pairs)
	}
}

func TestSerializeResourceAttrs_RoundTrip(t *testing.T) {
	in := "enduser.id=jdoe,deployment.environment=prod"
	out := serializeResourceAttrs(parseResourceAttrs(in))
	if out != in {
		t.Errorf("round trip: got %q, want %q", out, in)
	}
}

func TestSetProjectName_InsertsFirstWhenAbsent(t *testing.T) {
	pairs := parseResourceAttrs("enduser.id=jdoe")
	pairs = setSubKey(pairs, SubKeyProjectName, "myproj")
	out := serializeResourceAttrs(pairs)
	want := "project.name=myproj,enduser.id=jdoe"
	if out != want {
		t.Errorf("got %q, want %q", out, want)
	}
}

func TestSetProjectName_UpdatesInPlaceWhenPresent(t *testing.T) {
	pairs := parseResourceAttrs("enduser.id=jdoe,project.name=old,deployment.environment=prod")
	pairs = setSubKey(pairs, SubKeyProjectName, "new")
	out := serializeResourceAttrs(pairs)
	want := "enduser.id=jdoe,project.name=new,deployment.environment=prod"
	if out != want {
		t.Errorf("got %q, want %q", out, want)
	}
}

func TestSetProjectName_OnEmptyList(t *testing.T) {
	pairs := parseResourceAttrs("")
	pairs = setSubKey(pairs, SubKeyProjectName, "myproj")
	out := serializeResourceAttrs(pairs)
	if out != "project.name=myproj" {
		t.Errorf("got %q", out)
	}
}

func TestGetSubKey(t *testing.T) {
	pairs := parseResourceAttrs("a=1,b=2")
	if v, ok := getSubKey(pairs, "b"); !ok || v != "2" {
		t.Errorf("b: got (%q,%v)", v, ok)
	}
	if _, ok := getSubKey(pairs, "missing"); ok {
		t.Error("missing key reported as present")
	}
}
