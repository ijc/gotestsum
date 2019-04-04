/*Package junitxml creates a JUnit XML report from a testjson.Execution.
 */
package junitxml

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gotest.tools/gotestsum/testjson"
)

// JUnitTestSuites is a collection of JUnit test suites.
type JUnitTestSuites struct {
	XMLName xml.Name `xml:"testsuites"`
	Suites  []JUnitTestSuite
}

// JUnitTestSuite is a single JUnit test suite which may contain many
// testcases.
type JUnitTestSuite struct {
	XMLName    xml.Name        `xml:"testsuite"`
	Tests      int             `xml:"tests,attr"`
	Failures   int             `xml:"failures,attr"`
	Time       string          `xml:"time,attr"`
	Name       string          `xml:"name,attr"`
	Properties []JUnitProperty `xml:"properties>property,omitempty"`
	TestCases  []JUnitTestCase
}

// JUnitTestCase is a single test case with its result.
type JUnitTestCase struct {
	XMLName     xml.Name          `xml:"testcase"`
	Classname   string            `xml:"classname,attr"`
	Name        string            `xml:"name,attr"`
	Time        string            `xml:"time,attr"`
	SkipMessage *JUnitSkipMessage `xml:"skipped,omitempty"`
	Failure     *JUnitFailure     `xml:"failure,omitempty"`
}

// JUnitSkipMessage contains the reason why a testcase was skipped.
type JUnitSkipMessage struct {
	Message string `xml:"message,attr"`
}

// JUnitProperty represents a key/value pair used to define properties.
type JUnitProperty struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

// JUnitFailure contains data related to a failed test.
type JUnitFailure struct {
	Message  string `xml:"message,attr"`
	Type     string `xml:"type,attr"`
	Contents string `xml:",chardata"`
}

// Write creates an XML document and writes it to out.
func Write(out io.Writer, exec *testjson.Execution, strip int, prefix string) error {
	return errors.Wrap(write(out, generate(exec, strip, prefix)), "failed to write JUnit XML")
}

func stripPathElements(pkgname string, strip int) string {
	if strip == 0 {
		return pkgname
	}
	elems := strings.Split(pkgname, "/")
	if strip > len(elems) {
		return ""
	}
	return path.Join(elems[strip:]...)
}

func mungePackageName(n string, strip int, prefix string) string {
	n = stripPathElements(n, strip)
	n = path.Join(prefix, n)
	// Junit assume "Java" style package names ("."-separated
	// hierarchy) and Jenkins tries to render as such, which for
	// packages like `github.com/foo/bar` can result in a display
	// hierarchy of `github` → `com/foo/bar`.
	//
	// To avoid this convert all `.` into `-` (which is not
	// normally allowed in a Go package name) and then all `/`
	// into `.`. Thus `github.com/foo/bar` becomes `github-com.foo.bar`.
	n = strings.Replace(n, ".", "-", -1)
	n = strings.Replace(n, "/", ".", -1)

	return n
}

func generate(exec *testjson.Execution, strip int, prefix string) JUnitTestSuites {
	version := goVersion()
	suites := JUnitTestSuites{}
	for _, pkgname := range exec.Packages() {
		pkg := exec.Package(pkgname)
		if x := os.Getenv("GOTESTSUM_SUITE"); x != "" {
			pkgname = x
		} else {
			pkgname = mungePackageName(pkgname, strip, prefix)
		}
		junitpkg := JUnitTestSuite{
			Name:       pkgname,
			Tests:      pkg.Total,
			Time:       formatDurationAsSeconds(pkg.Elapsed()),
			Properties: packageProperties(version),
			TestCases:  packageTestCases(pkg, strip, prefix),
			Failures:   len(pkg.Failed),
		}
		suites.Suites = append(suites.Suites, junitpkg)
	}
	return suites
}

func formatDurationAsSeconds(d time.Duration) string {
	return fmt.Sprintf("%f", d.Seconds())
}

func packageProperties(goVersion string) []JUnitProperty {
	return []JUnitProperty{
		{Name: "go.version", Value: goVersion},
	}
}

// goVersion returns the version as reported by the go binary in PATH. This
// version will not be the same as runtime.Version, which is always the version
// of go used to build the gotestsum binary.
//
// To skip the os/exec call set the GOVERSION environment variable to the
// desired value.
func goVersion() string {
	if version, ok := os.LookupEnv("GOVERSION"); ok {
		return version
	}
	logrus.Debugf("exec: go version")
	cmd := exec.Command("go", "version")
	out, err := cmd.Output()
	if err != nil {
		logrus.WithError(err).Warn("failed to lookup go version for junit xml")
		return "unknown"
	}
	return strings.TrimPrefix(strings.TrimSpace(string(out)), "go version ")
}

func packageTestCases(pkg *testjson.Package, strip int, prefix string) []JUnitTestCase {
	cases := []JUnitTestCase{}

	if pkg.TestMainFailed() {
		jtc := newJUnitTestCase(testjson.TestCase{
			Test: "TestMain",
		}, strip, prefix)
		jtc.Failure = &JUnitFailure{
			Message:  "Failed",
			Contents: pkg.Output(""),
		}
		cases = append(cases, jtc)
	}

	for _, tc := range pkg.Failed {
		jtc := newJUnitTestCase(tc, strip, prefix)
		jtc.Failure = &JUnitFailure{
			Message:  "Failed",
			Contents: pkg.Output(tc.Test),
		}
		cases = append(cases, jtc)
	}

	for _, tc := range pkg.Skipped {
		jtc := newJUnitTestCase(tc, strip, prefix)
		jtc.SkipMessage = &JUnitSkipMessage{Message: pkg.Output(tc.Test)}
		cases = append(cases, jtc)
	}

	for _, tc := range pkg.Passed {
		jtc := newJUnitTestCase(tc, strip, prefix)
		cases = append(cases, jtc)
	}
	return cases
}

func newJUnitTestCase(tc testjson.TestCase, strip int, prefix string) JUnitTestCase {
	return JUnitTestCase{
		Classname: mungePackageName(tc.Package, strip, prefix),
		Name:      tc.Test,
		Time:      formatDurationAsSeconds(tc.Elapsed),
	}
}

func write(out io.Writer, suites JUnitTestSuites) error {
	doc, err := xml.MarshalIndent(suites, "", "\t")
	if err != nil {
		return err
	}
	_, err = out.Write([]byte(xml.Header))
	if err != nil {
		return err
	}
	_, err = out.Write(doc)
	return err
}
