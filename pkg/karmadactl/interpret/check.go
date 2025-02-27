package interpret

import (
	"context"
	"fmt"
	"strings"
	"time"

	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/cli-runtime/pkg/resource"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/karmada-io/karmada/pkg/resourceinterpreter/customized/declarative/luavm"
)

func (o *Options) runCheck() error {
	w := printers.GetNewTabWriter(o.Out)
	defer w.Flush()

	failed := false

	err := o.CustomizationResult.Visit(func(info *resource.Info, _ error) error {
		var visitErr error
		fmt.Fprintln(w, "-----------------------------------")

		source := info.Source
		if info.Name != "" {
			source = info.Name
		}
		fmt.Fprintf(w, "SOURCE: %s\n", source)

		customization, visitErr := asResourceInterpreterCustomization(info.Object)
		if visitErr != nil {
			failed = true
			fmt.Fprintf(w, "%v\n", visitErr)
			return nil
		}

		kind := customization.Spec.Target.Kind
		if kind == "" {
			failed = true
			fmt.Fprintln(w, "target.kind no set")
			return nil
		}
		apiVersion := customization.Spec.Target.APIVersion
		if apiVersion == "" {
			failed = true
			fmt.Fprintln(w, "target.apiVersion no set")
			return nil
		}

		fmt.Fprintf(w, "TARGET: %s %s\t\n", apiVersion, kind)
		fmt.Fprintf(w, "RULERS:\n")
		for _, r := range o.Rules {
			fmt.Fprintf(w, "    %s:\t", r.Name())

			script := r.GetScript(customization)
			if script == "" {
				fmt.Fprintln(w, "UNSET")
				continue
			}
			checkErr := checkScrip(script)
			if checkErr != nil {
				failed = true
				fmt.Fprintf(w, "%s: %s\t\n", "ERROR", strings.TrimSpace(checkErr.Error()))
				continue
			}

			fmt.Fprintln(w, "PASS")
		}
		return nil
	})
	if err != nil {
		return err
	}
	if failed {
		// As failed infos are printed above. So don't print it again.
		return cmdutil.ErrExit
	}
	return nil
}

func checkScrip(script string) error {
	ctx, cancel := context.WithTimeout(context.TODO(), time.Second)
	defer cancel()
	l, err := luavm.NewWithContext(ctx)
	if err != nil {
		return err
	}
	defer l.Close()
	_, err = l.LoadString(script)
	return err
}
